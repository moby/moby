package memberlist

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/armon/go-metrics"
	"github.com/hashicorp/go-msgpack/codec"
)

// This is the minimum and maximum protocol version that we can
// _understand_. We're allowed to speak at any version within this
// range. This range is inclusive.
const (
	ProtocolVersionMin uint8 = 1
	ProtocolVersionMax       = 2
)

// messageType is an integer ID of a type of message that can be received
// on network channels from other members.
type messageType uint8

// The list of available message types.
const (
	pingMsg messageType = iota
	indirectPingMsg
	ackRespMsg
	suspectMsg
	aliveMsg
	deadMsg
	pushPullMsg
	compoundMsg
	userMsg // User mesg, not handled by us
	compressMsg
	encryptMsg
)

// compressionType is used to specify the compression algorithm
type compressionType uint8

const (
	lzwAlgo compressionType = iota
)

const (
	MetaMaxSize            = 512 // Maximum size for node meta data
	compoundHeaderOverhead = 2   // Assumed header overhead
	compoundOverhead       = 2   // Assumed overhead per entry in compoundHeader
	udpBufSize             = 65536
	udpRecvBuf             = 2 * 1024 * 1024
	udpSendBuf             = 1400
	userMsgOverhead        = 1
	blockingWarning        = 10 * time.Millisecond // Warn if a UDP packet takes this long to process
	maxPushStateBytes      = 10 * 1024 * 1024
)

// ping request sent directly to node
type ping struct {
	SeqNo uint32

	// Node is sent so the target can verify they are
	// the intended recipient. This is to protect again an agent
	// restart with a new name.
	Node string
}

// indirect ping sent to an indirect ndoe
type indirectPingReq struct {
	SeqNo  uint32
	Target []byte
	Port   uint16
	Node   string
}

// ack response is sent for a ping
type ackResp struct {
	SeqNo uint32
}

// suspect is broadcast when we suspect a node is dead
type suspect struct {
	Incarnation uint32
	Node        string
	From        string // Include who is suspecting
}

// alive is broadcast when we know a node is alive.
// Overloaded for nodes joining
type alive struct {
	Incarnation uint32
	Node        string
	Addr        []byte
	Port        uint16
	Meta        []byte

	// The versions of the protocol/delegate that are being spoken, order:
	// pmin, pmax, pcur, dmin, dmax, dcur
	Vsn []uint8
}

// dead is broadcast when we confirm a node is dead
// Overloaded for nodes leaving
type dead struct {
	Incarnation uint32
	Node        string
	From        string // Include who is suspecting
}

// pushPullHeader is used to inform the
// otherside how many states we are transfering
type pushPullHeader struct {
	Nodes        int
	UserStateLen int  // Encodes the byte lengh of user state
	Join         bool // Is this a join request or a anti-entropy run
}

// pushNodeState is used for pushPullReq when we are
// transfering out node states
type pushNodeState struct {
	Name        string
	Addr        []byte
	Port        uint16
	Meta        []byte
	Incarnation uint32
	State       nodeStateType
	Vsn         []uint8 // Protocol versions
}

// compress is used to wrap an underlying payload
// using a specified compression algorithm
type compress struct {
	Algo compressionType
	Buf  []byte
}

// msgHandoff is used to transfer a message between goroutines
type msgHandoff struct {
	msgType messageType
	buf     []byte
	from    net.Addr
}

// encryptionVersion returns the encryption version to use
func (m *Memberlist) encryptionVersion() encryptionVersion {
	switch m.ProtocolVersion() {
	case 1:
		return 0
	default:
		return 1
	}
}

// setUDPRecvBuf is used to resize the UDP receive window. The function
// attempts to set the read buffer to `udpRecvBuf` but backs off until
// the read buffer can be set.
func setUDPRecvBuf(c *net.UDPConn) {
	size := udpRecvBuf
	for {
		if err := c.SetReadBuffer(size); err == nil {
			break
		}
		size = size / 2
	}
}

// tcpListen listens for and handles incoming connections
func (m *Memberlist) tcpListen() {
	for {
		conn, err := m.tcpListener.AcceptTCP()
		if err != nil {
			if m.shutdown {
				break
			}
			m.logger.Printf("[ERR] memberlist: Error accepting TCP connection: %s", err)
			continue
		}
		go m.handleConn(conn)
	}
}

// handleConn handles a single incoming TCP connection
func (m *Memberlist) handleConn(conn *net.TCPConn) {
	m.logger.Printf("[DEBUG] memberlist: Responding to push/pull sync with: %s", conn.RemoteAddr())
	defer conn.Close()
	metrics.IncrCounter([]string{"memberlist", "tcp", "accept"}, 1)

	join, remoteNodes, userState, err := m.readRemoteState(conn)
	if err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to receive remote state: %s", err)
		return
	}

	if err := m.sendLocalState(conn, join); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to push local state: %s", err)
	}

	if err := m.verifyProtocol(remoteNodes); err != nil {
		m.logger.Printf("[ERR] memberlist: Push/pull verification failed: %s", err)
		return
	}

	// Invoke the merge delegate if any
	if join && m.config.Merge != nil {
		nodes := make([]*Node, len(remoteNodes))
		for idx, n := range remoteNodes {
			nodes[idx] = &Node{
				Name: n.Name,
				Addr: n.Addr,
				Port: n.Port,
				Meta: n.Meta,
				PMin: n.Vsn[0],
				PMax: n.Vsn[1],
				PCur: n.Vsn[2],
				DMin: n.Vsn[3],
				DMax: n.Vsn[4],
				DCur: n.Vsn[5],
			}
		}
		if m.config.Merge.NotifyMerge(nodes) {
			m.logger.Printf("[WARN] memberlist: Cluster merge canceled")
			return
		}
	}

	// Merge the membership state
	m.mergeState(remoteNodes)

	// Invoke the delegate for user state
	if m.config.Delegate != nil {
		m.config.Delegate.MergeRemoteState(userState, join)
	}
}

// udpListen listens for and handles incoming UDP packets
func (m *Memberlist) udpListen() {
	var n int
	var addr net.Addr
	var err error
	var lastPacket time.Time
	for {
		// Do a check for potentially blocking operations
		if !lastPacket.IsZero() && time.Now().Sub(lastPacket) > blockingWarning {
			diff := time.Now().Sub(lastPacket)
			m.logger.Printf(
				"[DEBUG] memberlist: Potential blocking operation. Last command took %v",
				diff)
		}

		// Create a new buffer
		// TODO: Use Sync.Pool eventually
		buf := make([]byte, udpBufSize)

		// Read a packet
		n, addr, err = m.udpListener.ReadFrom(buf)
		if err != nil {
			if m.shutdown {
				break
			}
			m.logger.Printf("[ERR] memberlist: Error reading UDP packet: %s", err)
			continue
		}

		// Check the length
		if n < 1 {
			m.logger.Printf("[ERR] memberlist: UDP packet too short (%d bytes). From: %s",
				len(buf), addr)
			continue
		}

		// Capture the current time
		lastPacket = time.Now()

		// Ingest this packet
		metrics.IncrCounter([]string{"memberlist", "udp", "received"}, float32(n))
		m.ingestPacket(buf[:n], addr)
	}
}

func (m *Memberlist) ingestPacket(buf []byte, from net.Addr) {
	// Check if encryption is enabled
	if m.config.EncryptionEnabled() {
		// Decrypt the payload
		plain, err := decryptPayload(m.config.Keyring.GetKeys(), buf, nil)
		if err != nil {
			m.logger.Printf("[ERR] memberlist: Decrypt packet failed: %v", err)
			return
		}

		// Continue processing the plaintext buffer
		buf = plain
	}

	// Handle the command
	m.handleCommand(buf, from)
}

func (m *Memberlist) handleCommand(buf []byte, from net.Addr) {
	// Decode the message type
	msgType := messageType(buf[0])
	buf = buf[1:]

	// Switch on the msgType
	switch msgType {
	case compoundMsg:
		m.handleCompound(buf, from)
	case compressMsg:
		m.handleCompressed(buf, from)

	case pingMsg:
		m.handlePing(buf, from)
	case indirectPingMsg:
		m.handleIndirectPing(buf, from)
	case ackRespMsg:
		m.handleAck(buf, from)

	case suspectMsg:
		fallthrough
	case aliveMsg:
		fallthrough
	case deadMsg:
		fallthrough
	case userMsg:
		select {
		case m.handoff <- msgHandoff{msgType, buf, from}:
		default:
			m.logger.Printf("[WARN] memberlist: UDP handler queue full, dropping message (%d)", msgType)
		}

	default:
		m.logger.Printf("[ERR] memberlist: UDP msg type (%d) not supported. From: %s", msgType, from)
	}
}

// udpHandler processes messages received over UDP, but is decoupled
// from the listener to avoid blocking the listener which may cause
// ping/ack messages to be delayed.
func (m *Memberlist) udpHandler() {
	for {
		select {
		case msg := <-m.handoff:
			msgType := msg.msgType
			buf := msg.buf
			from := msg.from

			switch msgType {
			case suspectMsg:
				m.handleSuspect(buf, from)
			case aliveMsg:
				m.handleAlive(buf, from)
			case deadMsg:
				m.handleDead(buf, from)
			case userMsg:
				m.handleUser(buf, from)
			default:
				m.logger.Printf("[ERR] memberlist: UDP msg type (%d) not supported. From: %s (handler)", msgType, from)
			}

		case <-m.shutdownCh:
			return
		}
	}
}

func (m *Memberlist) handleCompound(buf []byte, from net.Addr) {
	// Decode the parts
	trunc, parts, err := decodeCompoundMessage(buf)
	if err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode compound request: %s", err)
		return
	}

	// Log any truncation
	if trunc > 0 {
		m.logger.Printf("[WARN] memberlist: Compound request had %d truncated messages", trunc)
	}

	// Handle each message
	for _, part := range parts {
		m.handleCommand(part, from)
	}
}

func (m *Memberlist) handlePing(buf []byte, from net.Addr) {
	var p ping
	if err := decode(buf, &p); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode ping request: %s", err)
		return
	}
	// If node is provided, verify that it is for us
	if p.Node != "" && p.Node != m.config.Name {
		m.logger.Printf("[WARN] memberlist: Got ping for unexpected node '%s'", p.Node)
		return
	}
	ack := ackResp{p.SeqNo}
	if err := m.encodeAndSendMsg(from, ackRespMsg, &ack); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to send ack: %s", err)
	}
}

func (m *Memberlist) handleIndirectPing(buf []byte, from net.Addr) {
	var ind indirectPingReq
	if err := decode(buf, &ind); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode indirect ping request: %s", err)
		return
	}

	// For proto versions < 2, there is no port provided. Mask old
	// behavior by using the configured port
	if m.ProtocolVersion() < 2 || ind.Port == 0 {
		ind.Port = uint16(m.config.BindPort)
	}

	// Send a ping to the correct host
	localSeqNo := m.nextSeqNo()
	ping := ping{SeqNo: localSeqNo, Node: ind.Node}
	destAddr := &net.UDPAddr{IP: ind.Target, Port: int(ind.Port)}

	// Setup a response handler to relay the ack
	respHandler := func() {
		ack := ackResp{ind.SeqNo}
		if err := m.encodeAndSendMsg(from, ackRespMsg, &ack); err != nil {
			m.logger.Printf("[ERR] memberlist: Failed to forward ack: %s", err)
		}
	}
	m.setAckHandler(localSeqNo, respHandler, m.config.ProbeTimeout)

	// Send the ping
	if err := m.encodeAndSendMsg(destAddr, pingMsg, &ping); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to send ping: %s", err)
	}
}

func (m *Memberlist) handleAck(buf []byte, from net.Addr) {
	var ack ackResp
	if err := decode(buf, &ack); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode ack response: %s", err)
		return
	}
	m.invokeAckHandler(ack.SeqNo)
}

func (m *Memberlist) handleSuspect(buf []byte, from net.Addr) {
	var sus suspect
	if err := decode(buf, &sus); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode suspect message: %s", err)
		return
	}
	m.suspectNode(&sus)
}

func (m *Memberlist) handleAlive(buf []byte, from net.Addr) {
	var live alive
	if err := decode(buf, &live); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode alive message: %s", err)
		return
	}

	// For proto versions < 2, there is no port provided. Mask old
	// behavior by using the configured port
	if m.ProtocolVersion() < 2 || live.Port == 0 {
		live.Port = uint16(m.config.BindPort)
	}

	m.aliveNode(&live, nil, false)
}

func (m *Memberlist) handleDead(buf []byte, from net.Addr) {
	var d dead
	if err := decode(buf, &d); err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decode dead message: %s", err)
		return
	}
	m.deadNode(&d)
}

// handleUser is used to notify channels of incoming user data
func (m *Memberlist) handleUser(buf []byte, from net.Addr) {
	d := m.config.Delegate
	if d != nil {
		d.NotifyMsg(buf)
	}
}

// handleCompressed is used to unpack a compressed message
func (m *Memberlist) handleCompressed(buf []byte, from net.Addr) {
	// Try to decode the payload
	payload, err := decompressPayload(buf)
	if err != nil {
		m.logger.Printf("[ERR] memberlist: Failed to decompress payload: %v", err)
		return
	}

	// Recursively handle the payload
	m.handleCommand(payload, from)
}

// encodeAndSendMsg is used to combine the encoding and sending steps
func (m *Memberlist) encodeAndSendMsg(to net.Addr, msgType messageType, msg interface{}) error {
	out, err := encode(msgType, msg)
	if err != nil {
		return err
	}
	if err := m.sendMsg(to, out.Bytes()); err != nil {
		return err
	}
	return nil
}

// sendMsg is used to send a UDP message to another host. It will opportunistically
// create a compoundMsg and piggy back other broadcasts
func (m *Memberlist) sendMsg(to net.Addr, msg []byte) error {
	// Check if we can piggy back any messages
	bytesAvail := udpSendBuf - len(msg) - compoundHeaderOverhead
	if m.config.EncryptionEnabled() {
		bytesAvail -= encryptOverhead(m.encryptionVersion())
	}
	extra := m.getBroadcasts(compoundOverhead, bytesAvail)

	// Fast path if nothing to piggypack
	if len(extra) == 0 {
		return m.rawSendMsg(to, msg)
	}

	// Join all the messages
	msgs := make([][]byte, 0, 1+len(extra))
	msgs = append(msgs, msg)
	msgs = append(msgs, extra...)

	// Create a compound message
	compound := makeCompoundMessage(msgs)

	// Send the message
	return m.rawSendMsg(to, compound.Bytes())
}

// rawSendMsg is used to send a UDP message to another host without modification
func (m *Memberlist) rawSendMsg(to net.Addr, msg []byte) error {
	// Check if we have compression enabled
	if m.config.EnableCompression {
		buf, err := compressPayload(msg)
		if err != nil {
			m.logger.Printf("[WARN] memberlist: Failed to compress payload: %v", err)
		} else {
			// Only use compression if it reduced the size
			if buf.Len() < len(msg) {
				msg = buf.Bytes()
			}
		}
	}

	// Check if we have encryption enabled
	if m.config.EncryptionEnabled() {
		// Encrypt the payload
		var buf bytes.Buffer
		primaryKey := m.config.Keyring.GetPrimaryKey()
		err := encryptPayload(m.encryptionVersion(), primaryKey, msg, nil, &buf)
		if err != nil {
			m.logger.Printf("[ERR] memberlist: Encryption of message failed: %v", err)
			return err
		}
		msg = buf.Bytes()
	}

	metrics.IncrCounter([]string{"memberlist", "udp", "sent"}, float32(len(msg)))
	_, err := m.udpListener.WriteTo(msg, to)
	return err
}

// sendState is used to initiate a push/pull over TCP with a remote node
func (m *Memberlist) sendAndReceiveState(addr []byte, port uint16, join bool) ([]pushNodeState, []byte, error) {
	// Attempt to connect
	dialer := net.Dialer{Timeout: m.config.TCPTimeout}
	dest := net.TCPAddr{IP: addr, Port: int(port)}
	conn, err := dialer.Dial("tcp", dest.String())
	if err != nil {
		return nil, nil, err
	}
	defer conn.Close()
	m.logger.Printf("[DEBUG] memberlist: Initiating push/pull sync with: %s", conn.RemoteAddr())
	metrics.IncrCounter([]string{"memberlist", "tcp", "connect"}, 1)

	// Send our state
	if err := m.sendLocalState(conn, join); err != nil {
		return nil, nil, err
	}

	// Read remote state
	_, remote, userState, err := m.readRemoteState(conn)
	if err != nil {
		err := fmt.Errorf("Reading remote state failed: %v", err)
		return nil, nil, err
	}

	// Return the remote state
	return remote, userState, nil
}

// sendLocalState is invoked to send our local state over a tcp connection
func (m *Memberlist) sendLocalState(conn net.Conn, join bool) error {
	// Setup a deadline
	conn.SetDeadline(time.Now().Add(m.config.TCPTimeout))

	// Prepare the local node state
	m.nodeLock.RLock()
	localNodes := make([]pushNodeState, len(m.nodes))
	for idx, n := range m.nodes {
		localNodes[idx].Name = n.Name
		localNodes[idx].Addr = n.Addr
		localNodes[idx].Port = n.Port
		localNodes[idx].Incarnation = n.Incarnation
		localNodes[idx].State = n.State
		localNodes[idx].Meta = n.Meta
		localNodes[idx].Vsn = []uint8{
			n.PMin, n.PMax, n.PCur,
			n.DMin, n.DMax, n.DCur,
		}
	}
	m.nodeLock.RUnlock()

	// Get the delegate state
	var userData []byte
	if m.config.Delegate != nil {
		userData = m.config.Delegate.LocalState(join)
	}

	// Create a bytes buffer writer
	bufConn := bytes.NewBuffer(nil)

	// Send our node state
	header := pushPullHeader{Nodes: len(localNodes), UserStateLen: len(userData), Join: join}
	hd := codec.MsgpackHandle{}
	enc := codec.NewEncoder(bufConn, &hd)

	// Begin state push
	if _, err := bufConn.Write([]byte{byte(pushPullMsg)}); err != nil {
		return err
	}

	if err := enc.Encode(&header); err != nil {
		return err
	}
	for i := 0; i < header.Nodes; i++ {
		if err := enc.Encode(&localNodes[i]); err != nil {
			return err
		}
	}

	// Write the user state as well
	if userData != nil {
		if _, err := bufConn.Write(userData); err != nil {
			return err
		}
	}

	// Get the send buffer
	sendBuf := bufConn.Bytes()

	// Check if compresion is enabled
	if m.config.EnableCompression {
		compBuf, err := compressPayload(bufConn.Bytes())
		if err != nil {
			m.logger.Printf("[ERROR] memberlist: Failed to compress local state: %v", err)
		} else {
			sendBuf = compBuf.Bytes()
		}
	}

	// Check if encryption is enabled
	if m.config.EncryptionEnabled() {
		crypt, err := m.encryptLocalState(sendBuf)
		if err != nil {
			m.logger.Printf("[ERROR] memberlist: Failed to encrypt local state: %v", err)
			return err
		}
		sendBuf = crypt
	}

	// Write out the entire send buffer
	metrics.IncrCounter([]string{"memberlist", "tcp", "sent"}, float32(len(sendBuf)))
	if _, err := conn.Write(sendBuf); err != nil {
		return err
	}
	return nil
}

// encryptLocalState is used to help encrypt local state before sending
func (m *Memberlist) encryptLocalState(sendBuf []byte) ([]byte, error) {
	var buf bytes.Buffer

	// Write the encryptMsg byte
	buf.WriteByte(byte(encryptMsg))

	// Write the size of the message
	sizeBuf := make([]byte, 4)
	encVsn := m.encryptionVersion()
	encLen := encryptedLength(encVsn, len(sendBuf))
	binary.BigEndian.PutUint32(sizeBuf, uint32(encLen))
	buf.Write(sizeBuf)

	// Write the encrypted cipher text to the buffer
	key := m.config.Keyring.GetPrimaryKey()
	err := encryptPayload(encVsn, key, sendBuf, buf.Bytes()[:5], &buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decryptRemoteState is used to help decrypt the remote state
func (m *Memberlist) decryptRemoteState(bufConn io.Reader) ([]byte, error) {
	// Read in enough to determine message length
	cipherText := bytes.NewBuffer(nil)
	cipherText.WriteByte(byte(encryptMsg))
	_, err := io.CopyN(cipherText, bufConn, 4)
	if err != nil {
		return nil, err
	}

	// Ensure we aren't asked to download too much. This is to guard against
	// an attack vector where a huge amount of state is sent
	moreBytes := binary.BigEndian.Uint32(cipherText.Bytes()[1:5])
	if moreBytes > maxPushStateBytes {
		return nil, fmt.Errorf("Remote node state is larger than limit (%d)", moreBytes)
	}

	// Read in the rest of the payload
	_, err = io.CopyN(cipherText, bufConn, int64(moreBytes))
	if err != nil {
		return nil, err
	}

	// Decrypt the cipherText
	dataBytes := cipherText.Bytes()[:5]
	cipherBytes := cipherText.Bytes()[5:]

	// Decrypt the payload
	keys := m.config.Keyring.GetKeys()
	return decryptPayload(keys, cipherBytes, dataBytes)
}

// recvRemoteState is used to read the remote state from a connection
func (m *Memberlist) readRemoteState(conn net.Conn) (bool, []pushNodeState, []byte, error) {
	// Setup a deadline
	conn.SetDeadline(time.Now().Add(m.config.TCPTimeout))

	// Created a buffered reader
	var bufConn io.Reader = bufio.NewReader(conn)

	// Read the message type
	buf := [1]byte{0}
	if _, err := bufConn.Read(buf[:]); err != nil {
		return false, nil, nil, err
	}
	msgType := messageType(buf[0])

	// Check if the message is encrypted
	if msgType == encryptMsg {
		if !m.config.EncryptionEnabled() {
			return false, nil, nil,
				fmt.Errorf("Remote state is encrypted and encryption is not configured")
		}

		plain, err := m.decryptRemoteState(bufConn)
		if err != nil {
			return false, nil, nil, err
		}

		// Reset message type and bufConn
		msgType = messageType(plain[0])
		bufConn = bytes.NewReader(plain[1:])
	} else if m.config.EncryptionEnabled() {
		return false, nil, nil,
			fmt.Errorf("Encryption is configured but remote state is not encrypted")
	}

	// Get the msgPack decoders
	hd := codec.MsgpackHandle{}
	dec := codec.NewDecoder(bufConn, &hd)

	// Check if we have a compressed message
	if msgType == compressMsg {
		var c compress
		if err := dec.Decode(&c); err != nil {
			return false, nil, nil, err
		}
		decomp, err := decompressBuffer(&c)
		if err != nil {
			return false, nil, nil, err
		}

		// Reset the message type
		msgType = messageType(decomp[0])

		// Create a new bufConn
		bufConn = bytes.NewReader(decomp[1:])

		// Create a new decoder
		dec = codec.NewDecoder(bufConn, &hd)
	}

	// Quit if not push/pull
	if msgType != pushPullMsg {
		err := fmt.Errorf("received invalid msgType (%d)", msgType)
		return false, nil, nil, err
	}

	// Read the push/pull header
	var header pushPullHeader
	if err := dec.Decode(&header); err != nil {
		return false, nil, nil, err
	}

	// Allocate space for the transfer
	remoteNodes := make([]pushNodeState, header.Nodes)

	// Try to decode all the states
	for i := 0; i < header.Nodes; i++ {
		if err := dec.Decode(&remoteNodes[i]); err != nil {
			return false, remoteNodes, nil, err
		}
	}

	// Read the remote user state into a buffer
	var userBuf []byte
	if header.UserStateLen > 0 {
		userBuf = make([]byte, header.UserStateLen)
		bytes, err := io.ReadAtLeast(bufConn, userBuf, header.UserStateLen)
		if err == nil && bytes != header.UserStateLen {
			err = fmt.Errorf(
				"Failed to read full user state (%d / %d)",
				bytes, header.UserStateLen)
		}
		if err != nil {
			return false, remoteNodes, nil, err
		}
	}

	// For proto versions < 2, there is no port provided. Mask old
	// behavior by using the configured port
	for idx := range remoteNodes {
		if m.ProtocolVersion() < 2 || remoteNodes[idx].Port == 0 {
			remoteNodes[idx].Port = uint16(m.config.BindPort)
		}
	}

	return header.Join, remoteNodes, userBuf, nil
}
