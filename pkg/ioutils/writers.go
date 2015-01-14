package ioutils

import (
	"fmt"
	"io"
)

type NopWriter struct{}

func (*NopWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}

type NopFlusher struct{}

func (f *NopFlusher) Flush() {}

type writeCloserWrapper struct {
	io.Writer
	closer func() error
}

func (r *writeCloserWrapper) Close() error {
	return r.closer()
}

func NewWriteCloserWrapper(r io.Writer, closer func() error) io.WriteCloser {
	return &writeCloserWrapper{
		Writer: r,
		closer: closer,
	}
}

type writeTransformWrapper struct {
	w           io.Writer
	transformer func(b []byte) ([]byte, error)
}

func (w *writeTransformWrapper) Write(b []byte) (int, error) {
	tb, err := w.transformer(b)
	if err != nil {
		return 0, err
	}

	n, err := w.w.Write(tb)
	if err != nil {
		return 0, err
	}
	if n != len(tb) {
		return 0, fmt.Errorf("short write")
	}

	return len(b), nil
}

func NewWriteTransformWrapper(w io.Writer, t func(b []byte) ([]byte, error)) *writeTransformWrapper {
	return &writeTransformWrapper{w: w, transformer: t}
}
