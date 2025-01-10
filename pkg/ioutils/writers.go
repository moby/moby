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
