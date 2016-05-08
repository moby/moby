package networkdb

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/hashicorp/go-msgpack/codec"
)

type messageType uint8

const (
	// For network join/leave event message
	networkEventMsg messageType = 1 + iota

	// For pushing/pulling network/node association state
	networkPushPullMsg

	// For table entry CRUD event message
	tableEventMsg

	// For building a compound message which packs many different
	// message types together
	compoundMsg

	// For syncing table entries in bulk b/w nodes.
	bulkSyncMsg
)

const (
	// Max udp message size chosen to avoid network packet
	// fragmentation.
	udpSendBuf = 1400

	// Compound message header overhead 1 byte(message type) + 4
	// bytes (num messages)
	compoundHeaderOverhead = 5

	// Overhead for each embedded message in a compound message 2
	// bytes (len of embedded message)
	compoundOverhead = 2
)

func decodeMessage(buf []byte, out interface{}) error {
	var handle codec.MsgpackHandle
	return codec.NewDecoder(bytes.NewReader(buf), &handle).Decode(out)
}

func encodeMessage(t messageType, msg interface{}) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	buf.WriteByte(uint8(t))

	handle := codec.MsgpackHandle{}
	encoder := codec.NewEncoder(buf, &handle)
	err := encoder.Encode(msg)
	return buf.Bytes(), err
}

// makeCompoundMessage takes a list of messages and generates
// a single compound message containing all of them
func makeCompoundMessage(msgs [][]byte) *bytes.Buffer {
	// Create a local buffer
	buf := bytes.NewBuffer(nil)

	// Write out the type
	buf.WriteByte(uint8(compoundMsg))

	// Write out the number of message
	binary.Write(buf, binary.BigEndian, uint32(len(msgs)))

	// Add the message lengths
	for _, m := range msgs {
		binary.Write(buf, binary.BigEndian, uint16(len(m)))
	}

	// Append the messages
	for _, m := range msgs {
		buf.Write(m)
	}

	return buf
}

// decodeCompoundMessage splits a compound message and returns
// the slices of individual messages. Also returns the number
// of truncated messages and any potential error
func decodeCompoundMessage(buf []byte) (trunc int, parts [][]byte, err error) {
	if len(buf) < 1 {
		err = fmt.Errorf("missing compound length byte")
		return
	}
	numParts := binary.BigEndian.Uint32(buf[0:4])
	buf = buf[4:]

	// Check we have enough bytes
	if len(buf) < int(numParts*2) {
		err = fmt.Errorf("truncated len slice")
		return
	}

	// Decode the lengths
	lengths := make([]uint16, numParts)
	for i := 0; i < int(numParts); i++ {
		lengths[i] = binary.BigEndian.Uint16(buf[i*2 : i*2+2])
	}
	buf = buf[numParts*2:]

	// Split each message
	for idx, msgLen := range lengths {
		if len(buf) < int(msgLen) {
			trunc = int(numParts) - idx
			return
		}

		// Extract the slice, seek past on the buffer
		slice := buf[:msgLen]
		buf = buf[msgLen:]
		parts = append(parts, slice)
	}
	return
}
