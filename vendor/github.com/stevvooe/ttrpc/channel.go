package ttrpc

import (
	"bufio"
	"context"
	"encoding/binary"
	"io"

	"github.com/pkg/errors"
)

const (
	messageHeaderLength = 10
	messageLengthMax    = 8 << 10
)

type messageType uint8

const (
	messageTypeRequest  messageType = 0x1
	messageTypeResponse messageType = 0x2
)

// messageHeader represents the fixed-length message header of 10 bytes sent
// with every request.
type messageHeader struct {
	Length   uint32      // length excluding this header. b[:4]
	StreamID uint32      // identifies which request stream message is a part of. b[4:8]
	Type     messageType // message type b[8]
	Flags    uint8       // reserved          b[9]
}

func readMessageHeader(p []byte, r io.Reader) (messageHeader, error) {
	_, err := io.ReadFull(r, p[:messageHeaderLength])
	if err != nil {
		return messageHeader{}, err
	}

	return messageHeader{
		Length:   binary.BigEndian.Uint32(p[:4]),
		StreamID: binary.BigEndian.Uint32(p[4:8]),
		Type:     messageType(p[8]),
		Flags:    p[9],
	}, nil
}

func writeMessageHeader(w io.Writer, p []byte, mh messageHeader) error {
	binary.BigEndian.PutUint32(p[:4], mh.Length)
	binary.BigEndian.PutUint32(p[4:8], mh.StreamID)
	p[8] = byte(mh.Type)
	p[9] = mh.Flags

	_, err := w.Write(p[:])
	return err
}

type channel struct {
	bw    *bufio.Writer
	br    *bufio.Reader
	hrbuf [messageHeaderLength]byte // avoid alloc when reading header
	hwbuf [messageHeaderLength]byte
}

func newChannel(w io.Writer, r io.Reader) *channel {
	return &channel{
		bw: bufio.NewWriter(w),
		br: bufio.NewReader(r),
	}
}

func (ch *channel) recv(ctx context.Context, p []byte) (messageHeader, error) {
	mh, err := readMessageHeader(ch.hrbuf[:], ch.br)
	if err != nil {
		return messageHeader{}, err
	}

	if mh.Length > uint32(len(p)) {
		return messageHeader{}, errors.Wrapf(io.ErrShortBuffer, "message length %v over buffer size %v", mh.Length, len(p))
	}

	if _, err := io.ReadFull(ch.br, p[:mh.Length]); err != nil {
		return messageHeader{}, errors.Wrapf(err, "failed reading message")
	}

	return mh, nil
}

func (ch *channel) send(ctx context.Context, streamID uint32, t messageType, p []byte) error {
	if err := writeMessageHeader(ch.bw, ch.hwbuf[:], messageHeader{Length: uint32(len(p)), StreamID: streamID, Type: t}); err != nil {
		return err
	}

	_, err := ch.bw.Write(p)
	if err != nil {
		return err
	}

	return ch.bw.Flush()
}
