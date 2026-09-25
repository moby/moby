package eventstream

import (
	"bytes"
	"io"
	"time"
)

// MessageSigner signs event stream message header and payload byte pairs.
// Each invocation chains off the previous signature.
type MessageSigner interface {
	SignMessage(headers, payload []byte, signingTime time.Time) ([]byte, error)
}

// SigningWriter wraps an io.WriteCloser and signs each event stream message
// frame written to it. Each Write call MUST contain exactly one complete
// encoded event stream message frame.
//
// The signing writer wraps each incoming frame in an outer event stream
// message with :date and :chunk-signature headers, then encodes the outer
// message to the underlying writer.
//
// Close sends a signed empty message to signal end-of-stream, then closes
// the underlying writer.
type SigningWriter struct {
	writer  io.WriteCloser
	signer  MessageSigner
	encoder *Encoder

	headersBuf bytes.Buffer
}

// NewSigningWriter returns a SigningWriter that signs frames and writes them
// to w.
func NewSigningWriter(w io.WriteCloser, signer MessageSigner) *SigningWriter {
	return &SigningWriter{
		writer:  w,
		signer:  signer,
		encoder: NewEncoder(),
	}
}

// Write signs a complete event stream message frame and writes the signed
// outer envelope to the underlying writer.
func (s *SigningWriter) Write(frame []byte) (int, error) {
	if err := s.signAndWrite(frame); err != nil {
		return 0, err
	}
	return len(frame), nil
}

// Close sends a signed empty message to signal end-of-stream, then closes
// the underlying writer.
func (s *SigningWriter) Close() error {
	if err := s.signAndWrite([]byte{}); err != nil {
		_ = s.writer.Close()
		return err
	}
	return s.writer.Close()
}

func (s *SigningWriter) signAndWrite(payload []byte) error {
	now := time.Now().UTC()

	var msg Message
	msg.Headers.Set(DateHeader, TimestampValue(now))
	msg.Payload = payload

	s.headersBuf.Reset()
	if err := EncodeHeaders(&s.headersBuf, msg.Headers); err != nil {
		return err
	}

	sig, err := s.signer.SignMessage(s.headersBuf.Bytes(), payload, now)
	if err != nil {
		return err
	}

	msg.Headers.Set(ChunkSignatureHeader, BytesValue(sig))

	return s.encoder.Encode(s.writer, msg)
}
