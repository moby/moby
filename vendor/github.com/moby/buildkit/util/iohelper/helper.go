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

type ReadCloser struct {
	io.ReadCloser
	CloseFunc func() error
}

func (rc *ReadCloser) Close() error {
	err1 := rc.ReadCloser.Close()
	err2 := rc.CloseFunc()
	if err1 != nil {
		return errors.Wrapf(err1, "failed to close: %v", err2)
	}
	return err2
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
