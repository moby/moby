package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"io"
	"sync/atomic"
)

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
