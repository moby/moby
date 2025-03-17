package main

import (
	"encoding/binary"
	"errors"
	"log"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

const (
	// DefaultConnTrackTimeout is the default timeout used for UDP connection
	// tracking.
	DefaultConnTrackTimeout = 90 * time.Second
	// UDPBufSize is the buffer size for the UDP proxy
	UDPBufSize = 65507
)

// A net.Addr where the IP is split into two fields so you can use it as a key
// in a map:
type connTrackKey struct {
	IPHigh uint64
	IPLow  uint64
	Port   int
}

func newConnTrackKey(addr *net.UDPAddr) *connTrackKey {
	if len(addr.IP) == net.IPv4len {
		return &connTrackKey{
			IPHigh: 0,
			IPLow:  uint64(binary.BigEndian.Uint32(addr.IP)),
			Port:   addr.Port,
		}
	}
	return &connTrackKey{
		IPHigh: binary.BigEndian.Uint64(addr.IP[:8]),
		IPLow:  binary.BigEndian.Uint64(addr.IP[8:]),
		Port:   addr.Port,
	}
}

type connTrackMap map[connTrackKey]*connTrackEntry

// connTrackEntry wraps a UDP connection to provide thread-safe [net.Conn.Write]
// and [net.Conn.Close] operations.
type connTrackEntry struct {
	conn  *net.UDPConn
	lastW time.Time
	// This lock should be held before calling Write or Close on the wrapped
	// net.UDPConn. Read can be called concurrently to these operations.
	//
	// Never lock mu without locking UDPProxy.connTrackLock first.
	mu sync.Mutex
}

func newConnTrackEntry(conn *net.UDPConn) *connTrackEntry {
	return &connTrackEntry{
		conn: conn,
		mu:   sync.Mutex{},
	}
}

func (cte *connTrackEntry) lastWrite() time.Time {
	cte.mu.Lock()
	defer cte.mu.Unlock()
	return cte.lastW
}

// UDPProxy is proxy for which handles UDP datagrams. It implements the Proxy
// interface to handle UDP traffic forwarding between the frontend and backend
// addresses.
type UDPProxy struct {
	listener         *net.UDPConn
	frontendAddr     *net.UDPAddr
	backendAddr      *net.UDPAddr
	connTrackTable   connTrackMap
	connTrackLock    sync.Mutex
	connTrackTimeout time.Duration
	ipVer            ipVersion
}

// NewUDPProxy creates a new UDPProxy.
func NewUDPProxy(listener *net.UDPConn, backendAddr *net.UDPAddr, ipVer ipVersion) (*UDPProxy, error) {
	return &UDPProxy{
		listener:         listener,
		frontendAddr:     listener.LocalAddr().(*net.UDPAddr),
		backendAddr:      backendAddr,
		connTrackTable:   make(connTrackMap),
		connTrackTimeout: DefaultConnTrackTimeout,
		ipVer:            ipVer,
	}, nil
}

func (proxy *UDPProxy) replyLoop(cte *connTrackEntry, serverAddr net.IP, clientAddr *net.UDPAddr, clientKey *connTrackKey) {
	defer func() {
		proxy.connTrackLock.Lock()
		delete(proxy.connTrackTable, *clientKey)
		cte.mu.Lock()
		proxy.connTrackLock.Unlock()
		cte.conn.Close()
	}()

	var oob []byte
	if proxy.ipVer == ip4 {
		cm := &ipv4.ControlMessage{Src: serverAddr}
		oob = cm.Marshal()
	} else {
		cm := &ipv6.ControlMessage{Src: serverAddr}
		oob = cm.Marshal()
	}

	readBuf := make([]byte, UDPBufSize)
	for {
		cte.conn.SetReadDeadline(time.Now().Add(proxy.connTrackTimeout))
	again:
		read, err := cte.conn.Read(readBuf)
		if err != nil {
			if err, ok := err.(*net.OpError); ok && err.Err == syscall.ECONNREFUSED {
				// This will happen if the last write failed
				// (e.g: nothing is actually listening on the
				// proxied port on the container), ignore it
				// and continue until DefaultConnTrackTimeout
				// expires:
				goto again
			}
			// If the UDP connection is one-sided (i.e. the backend never sends
			// replies), the connTrackEntry should not be GC'd until no writes
			// happen for proxy.connTrackTimeout.
			//
			// Since the ReadDeadline is set to proxy.connTrackTimeout, in such
			// case, the connTrackEntry will be GC'd at most after 2 * proxy.connTrackTimeout.
			if errors.Is(err, os.ErrDeadlineExceeded) && time.Since(cte.lastWrite()) < proxy.connTrackTimeout {
				continue
			}
			return
		}
		for i := 0; i != read; {
			written, _, err := proxy.listener.WriteMsgUDP(readBuf[i:read], oob, clientAddr)
			if err != nil {
				return
			}
			i += written
		}
	}
}

// Run starts forwarding the traffic using UDP.
func (proxy *UDPProxy) Run() {
	readBuf := make([]byte, UDPBufSize)
	var oob []byte
	if proxy.ipVer == ip4 {
		oob = ipv4.NewControlMessage(ipv4.FlagDst)
	} else {
		oob = ipv6.NewControlMessage(ipv6.FlagDst)
	}

	for {
		read, _, _, from, err := proxy.listener.ReadMsgUDP(readBuf, oob)
		if err != nil {
			// The frontend listener socket might be closed by the signal
			// handler. In that case, don't log anything - it's not an error.
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("Stopping proxy on udp/%v for udp/%v (%s)", proxy.frontendAddr, proxy.backendAddr, err)
			}
			break
		}

		fromKey := newConnTrackKey(from)
		proxy.connTrackLock.Lock()
		cte, hit := proxy.connTrackTable[*fromKey]
		if !hit {
			proxyConn, err := net.DialUDP("udp", nil, proxy.backendAddr)
			if err != nil {
				log.Printf("Can't proxy a datagram to udp/%s: %s\n", proxy.backendAddr, err)
				proxy.connTrackLock.Unlock()
				continue
			}
			cte = newConnTrackEntry(proxyConn)
			proxy.connTrackTable[*fromKey] = cte

			daddr, err := readDestFromCmsg(oob, proxy.ipVer)
			if err != nil {
				log.Printf("Failed to parse control message: %v", err)
				proxy.connTrackLock.Unlock()
				continue
			}

			go proxy.replyLoop(cte, daddr, from, fromKey)
		}
		cte.mu.Lock()
		proxy.connTrackLock.Unlock()
		cte.conn.SetWriteDeadline(time.Now().Add(proxy.connTrackTimeout))
		for i := 0; i != read; {
			written, err := cte.conn.Write(readBuf[i:read])
			if err != nil {
				log.Printf("Can't proxy a datagram to udp/%s: %s\n", proxy.backendAddr, err)
				break
			}
			i += written
			cte.lastW = time.Now()
		}
		cte.mu.Unlock()
	}
}

func readDestFromCmsg(oob []byte, ipVer ipVersion) (_ net.IP, err error) {
	defer func() {
		// In case of partial upgrade / downgrade, docker-proxy could read
		// control messages from a socket which doesn't have the sockopt
		// IP_PKTINFO enabled. In that case, the control message will be all-0
		// and Go's ControlMessage.Parse() will report an 'invalid header
		// length' error. In that case, ignore the error and return an empty
		// daddr - the kernel will pick a source address for us anyway (but
		// maybe it'll be the wrong one).
		if err != nil && err.Error() == "invalid header length" {
			err = nil
		}
	}()

	if ipVer == ip4 {
		cm := &ipv4.ControlMessage{}
		if err := cm.Parse(oob); err != nil {
			return nil, err
		}
		return cm.Dst, nil
	}

	cm := &ipv6.ControlMessage{}
	if err := cm.Parse(oob); err != nil {
		return nil, err
	}
	return cm.Dst, nil
}

// Close ungracefully stops forwarding the traffic.
func (proxy *UDPProxy) Close() {
	proxy.listener.Close()
	proxy.connTrackLock.Lock()
	defer proxy.connTrackLock.Unlock()
	for _, cte := range proxy.connTrackTable {
		// Unlike the GC logic in replyLoop, we want to close the connections
		// immediately, even if there are pending and in-progress writes. So no
		// need to lock cte.mu here.
		cte.conn.Close()
	}
}
