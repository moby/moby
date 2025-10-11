package stdcopymux

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"sync"

	"github.com/moby/moby/client/pkg/stdcopy"
)

const (
	stdWriterPrefixLen = 8
	stdWriterFdIndex   = 0
	stdWriterSizeIndex = 4
)

var bufPool = &sync.Pool{New: func() any { return bytes.NewBuffer(nil) }}

// stdWriter is wrapper of io.Writer with extra customized info.
type stdWriter struct {
	io.Writer
	prefix byte
}

// Write sends the buffer to the underlying writer.
// It inserts the prefix header before the buffer,
// so [StdCopy] knows where to multiplex the output.
//
// It implements [io.Writer].
func (w *stdWriter) Write(p []byte) (int, error) {
	if w == nil || w.Writer == nil {
		return 0, errors.New("writer not instantiated")
	}
	if p == nil {
		return 0, nil
	}

	header := [stdWriterPrefixLen]byte{stdWriterFdIndex: w.prefix}
	binary.BigEndian.PutUint32(header[stdWriterSizeIndex:], uint32(len(p)))
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Write(header[:])
	buf.Write(p)

	n, err := w.Writer.Write(buf.Bytes())
	n -= stdWriterPrefixLen
	if n < 0 {
		n = 0
	}

	buf.Reset()
	bufPool.Put(buf)
	return n, err
}

// NewStdWriter instantiates a new writer using a custom format to multiplex
// multiple streams to a single writer. All messages written using this writer
// are encapsulated using a custom format, and written to the underlying
// stream "w".
//
// Writers created through NewStdWriter allow for multiple write streams
// (e.g., stdout ([Stdout]) and stderr ([Stderr]) to be multiplexed into a
// single connection. "streamType" indicates the type of stream to encapsulate,
// commonly, [Stdout] or [Stderr]. The [Systemerr] stream can be used to
// include server-side errors in the stream. Information on this stream
// is returned as an error by [StdCopy] and terminates processing the
// stream.
//
// The [Stdin] stream is present for completeness and should generally
// NOT be used. It is output on [Stdout] when reading the stream with
// [StdCopy].
//
// All streams must share the same underlying [io.Writer] to ensure proper
// multiplexing. Each call to NewStdWriter wraps that shared writer with
// a header indicating the target stream.
func NewStdWriter(w io.Writer, streamType stdcopy.StdType) io.Writer {
	return &stdWriter{
		Writer: w,
		prefix: byte(streamType),
	}
}
