package stdcopy

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// StdType is the type of standard stream
// a writer can multiplex to.
type StdType byte

const (
	Stdin     StdType = 0 // Stdin represents standard input stream. It is present for completeness and should NOT be used. When reading the stream with [StdCopy] it is output on [Stdout].
	Stdout    StdType = 1 // Stdout represents standard output stream.
	Stderr    StdType = 2 // Stderr represents standard error steam.
	Systemerr StdType = 3 // Systemerr represents errors originating from the system. When reading the stream with [StdCopy] it is returned as an error.
)

const (
	stdWriterPrefixLen = 8
	stdWriterFdIndex   = 0
	stdWriterSizeIndex = 4

	startingBufLen = 32*1024 + stdWriterPrefixLen + 1
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
func NewStdWriter(w io.Writer, streamType StdType) io.Writer {
	return &stdWriter{
		Writer: w,
		prefix: byte(streamType),
	}
}

// StdCopy is a modified version of [io.Copy] to de-multiplex messages
// from "multiplexedSource" and copy them to destination streams
// "destOut" and "destErr".
//
// StdCopy demultiplexes "multiplexedSource", assuming that it contains
// two streams, previously multiplexed using a writer created with
// [NewStdWriter].
//
// As it reads from "multiplexedSource", StdCopy writes [Stdout] messages
// to "destOut", and [Stderr] message to "destErr]. For backward-compatibility,
// [Stdin] messages are output to "destOut". The [Systemerr] stream provides
// errors produced by the daemon. It is returned as an error, and terminates
// processing the stream.
//
// StdCopy it reads until it hits [io.EOF] on "multiplexedSource", after
// which it returns a nil error. In other words: any error returned indicates
// a real underlying error, which may be when an unknown [StdType] stream
// is received.
//
// The "written" return holds the total number of bytes written to "destOut"
// and "destErr" combined.
func StdCopy(destOut, destErr io.Writer, multiplexedSource io.Reader) (written int64, _ error) {
	var (
		buf       = make([]byte, startingBufLen)
		bufLen    = len(buf)
		nr, nw    int
		err       error
		out       io.Writer
		frameSize int
	)

	for {
		// Make sure we have at least a full header
		for nr < stdWriterPrefixLen {
			var nr2 int
			nr2, err = multiplexedSource.Read(buf[nr:])
			nr += nr2
			if errors.Is(err, io.EOF) {
				if nr < stdWriterPrefixLen {
					return written, nil
				}
				break
			}
			if err != nil {
				return 0, err
			}
		}

		// Check the first byte to know where to write
		stream := StdType(buf[stdWriterFdIndex])
		switch stream {
		case Stdin:
			fallthrough
		case Stdout:
			// Write on stdout
			out = destOut
		case Stderr:
			// Write on stderr
			out = destErr
		case Systemerr:
			// If we're on Systemerr, we won't write anywhere.
			// NB: if this code changes later, make sure you don't try to write
			// to outstream if Systemerr is the stream
			out = nil
		default:
			return 0, fmt.Errorf("unrecognized stream: %d", stream)
		}

		// Retrieve the size of the frame
		frameSize = int(binary.BigEndian.Uint32(buf[stdWriterSizeIndex : stdWriterSizeIndex+4]))

		// Check if the buffer is big enough to read the frame.
		// Extend it if necessary.
		if frameSize+stdWriterPrefixLen > bufLen {
			buf = append(buf, make([]byte, frameSize+stdWriterPrefixLen-bufLen+1)...)
			bufLen = len(buf)
		}

		// While the amount of bytes read is less than the size of the frame + header, we keep reading
		for nr < frameSize+stdWriterPrefixLen {
			var nr2 int
			nr2, err = multiplexedSource.Read(buf[nr:])
			nr += nr2
			if errors.Is(err, io.EOF) {
				if nr < frameSize+stdWriterPrefixLen {
					return written, nil
				}
				break
			}
			if err != nil {
				return 0, err
			}
		}

		// we might have an error from the source mixed up in our multiplexed
		// stream. if we do, return it.
		if stream == Systemerr {
			return written, fmt.Errorf("error from daemon in stream: %s", string(buf[stdWriterPrefixLen:frameSize+stdWriterPrefixLen]))
		}

		// Write the retrieved frame (without header)
		nw, err = out.Write(buf[stdWriterPrefixLen : frameSize+stdWriterPrefixLen])
		if err != nil {
			return 0, err
		}

		// If the frame has not been fully written: error
		if nw != frameSize {
			return 0, io.ErrShortWrite
		}
		written += int64(nw)

		// Move the rest of the buffer to the beginning
		copy(buf, buf[frameSize+stdWriterPrefixLen:])
		// Move the index
		nr -= frameSize + stdWriterPrefixLen
	}
}
