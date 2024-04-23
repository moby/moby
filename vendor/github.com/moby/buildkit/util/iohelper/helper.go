package iohelper

import (
	"io"
	"sync"

	"github.com/pkg/errors"
)

type NopWriteCloser struct {
	io.Writer
}

func (w *NopWriteCloser) Close() error {
	return nil
}

type closeFunc func() error

func (c closeFunc) Close() error {
	return c()
}

// WithCloser returns a ReadCloser with additional closer function.
func WithCloser(r io.ReadCloser, closer func() error) io.ReadCloser {
	var f closeFunc = func() error {
		err1 := r.Close()
		err2 := closer()
		if err1 != nil {
			return errors.Wrapf(err1, "failed to close: %v", err2)
		}
		return err2
	}
	return &readCloser{
		Reader: r,
		Closer: f,
	}
}

type WriteCloser struct {
	io.WriteCloser
	CloseFunc func() error
}

func (wc *WriteCloser) Close() error {
	err1 := wc.WriteCloser.Close()
	err2 := wc.CloseFunc()
	if err1 != nil {
		return errors.Wrapf(err1, "failed to close: %v", err2)
	}
	return err2
}

type Counter struct {
	n  int64
	mu sync.Mutex
}

func (c *Counter) Write(p []byte) (n int, err error) {
	c.mu.Lock()
	c.n += int64(len(p))
	c.mu.Unlock()
	return len(p), nil
}

func (c *Counter) Size() (n int64) {
	c.mu.Lock()
	n = c.n
	c.mu.Unlock()
	return
}

type ReaderAtCloser interface {
	io.ReaderAt
	io.Closer
	Size() int64
}

// ReadCloser returns a ReadCloser from ReaderAtCloser.
func ReadCloser(in ReaderAtCloser) io.ReadCloser {
	return &readCloser{
		Reader: io.NewSectionReader(in, 0, in.Size()),
		Closer: in,
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}
