package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"io"
	"sync/atomic"
)

// NopWriter represents a type which write operation is nop.
//
// Deprecated: use [io.Discard] instead. This type will be removed in the next release.
type NopWriter struct{}

func (*NopWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

// NopWriteCloser returns a nopWriteCloser.
//
// Deprecated: This function is no longer used and will be removed in the next release.
func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}

// NopFlusher represents a type which flush operation is nop.
//
// Deprecated: NopFlusher is only used internally and will be removed in the next release.
type NopFlusher = nopFlusher

type writeCloserWrapper struct {
	io.Writer
	closer func() error
	closed atomic.Bool
}

func (r *writeCloserWrapper) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		subsequentCloseWarn("WriteCloserWrapper")
		return nil
	}
	return r.closer()
}

// NewWriteCloserWrapper returns a new io.WriteCloser.
func NewWriteCloserWrapper(r io.Writer, closer func() error) io.WriteCloser {
	return &writeCloserWrapper{
		Writer: r,
		closer: closer,
	}
}

// WriteCounter wraps a concrete io.Writer and hold a count of the number
// of bytes written to the writer during a "session".
// This can be convenient when write return is masked
// (e.g., json.Encoder.Encode())
//
// Deprecated: this type is no longer used and will be removed in the next release.
type WriteCounter struct {
	Count  int64
	Writer io.Writer
}

// NewWriteCounter returns a new WriteCounter.
//
// Deprecated: this function is no longer used and will be removed in the next release.
func NewWriteCounter(w io.Writer) *WriteCounter {
	return &WriteCounter{
		Writer: w,
	}
}

func (wc *WriteCounter) Write(p []byte) (count int, err error) {
	count, err = wc.Writer.Write(p)
	wc.Count += int64(count)
	return
}
