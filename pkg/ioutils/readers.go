package ioutils // import "github.com/docker/docker/pkg/ioutils"

import (
	"context"
	"io"
	"runtime/debug"
	"sync/atomic"

	// make sure crypto.SHA256, crypto.sha512 and crypto.SHA384 are registered
	// TODO remove once https://github.com/opencontainers/go-digest/pull/64 is merged.
	_ "crypto/sha256"
	_ "crypto/sha512"

	"github.com/sirupsen/logrus"
)

// ReadCloserWrapper wraps an io.Reader, and implements an io.ReadCloser
// It calls the given callback function when closed. It should be constructed
// with NewReadCloserWrapper
type ReadCloserWrapper struct {
	io.Reader
	closer func() error
	closed atomic.Bool
}

// Close calls back the passed closer function
func (r *ReadCloserWrapper) Close() error {
	if !r.closed.CompareAndSwap(false, true) {
		subsequentCloseWarn("ReadCloserWrapper")
		return nil
	}
	return r.closer()
}

// NewReadCloserWrapper returns a new io.ReadCloser.
func NewReadCloserWrapper(r io.Reader, closer func() error) io.ReadCloser {
	return &ReadCloserWrapper{
		Reader: r,
		closer: closer,
	}
}

type readerErrWrapper struct {
	reader io.Reader
	closer func()
}

func (r *readerErrWrapper) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if err != nil {
		r.closer()
	}
	return n, err
}

// NewReaderErrWrapper returns a new io.Reader.
func NewReaderErrWrapper(r io.Reader, closer func()) io.Reader {
	return &readerErrWrapper{
		reader: r,
		closer: closer,
	}
}

// OnEOFReader wraps an io.ReadCloser and a function
// the function will run at the end of file or close the file.
type OnEOFReader struct {
	Rc io.ReadCloser
	Fn func()
}

func (r *OnEOFReader) Read(p []byte) (n int, err error) {
	n, err = r.Rc.Read(p)
	if err == io.EOF {
		r.runFunc()
	}
	return
}

// Close closes the file and run the function.
func (r *OnEOFReader) Close() error {
	err := r.Rc.Close()
	r.runFunc()
	return err
}

func (r *OnEOFReader) runFunc() {
	if fn := r.Fn; fn != nil {
		fn()
		r.Fn = nil
	}
}

// cancelReadCloser wraps an io.ReadCloser with a context for cancelling read
// operations.
type cancelReadCloser struct {
	cancel func()
	pR     *io.PipeReader // Stream to read from
	pW     *io.PipeWriter
	closed atomic.Bool
}

// NewCancelReadCloser creates a wrapper that closes the ReadCloser when the
// context is cancelled. The returned io.ReadCloser must be closed when it is
// no longer needed.
func NewCancelReadCloser(ctx context.Context, in io.ReadCloser) io.ReadCloser {
	pR, pW := io.Pipe()

	// Create a context used to signal when the pipe is closed
	doneCtx, cancel := context.WithCancel(context.Background())

	p := &cancelReadCloser{
		cancel: cancel,
		pR:     pR,
		pW:     pW,
	}

	go func() {
		_, err := io.Copy(pW, in)
		select {
		case <-ctx.Done():
			// If the context was closed, p.closeWithError
			// was already called. Calling it again would
			// change the error that Read returns.
		default:
			p.closeWithError(err)
		}
		in.Close()
	}()
	go func() {
		for {
			select {
			case <-ctx.Done():
				p.closeWithError(ctx.Err())
			case <-doneCtx.Done():
				return
			}
		}
	}()

	return p
}

// Read wraps the Read method of the pipe that provides data from the wrapped
// ReadCloser.
func (p *cancelReadCloser) Read(buf []byte) (n int, err error) {
	return p.pR.Read(buf)
}

// closeWithError closes the wrapper and its underlying reader. It will
// cause future calls to Read to return err.
func (p *cancelReadCloser) closeWithError(err error) {
	p.pW.CloseWithError(err)
	p.cancel()
}

// Close closes the wrapper its underlying reader. It will cause
// future calls to Read to return io.EOF.
func (p *cancelReadCloser) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		subsequentCloseWarn("cancelReadCloser")
		return nil
	}
	p.closeWithError(io.EOF)
	return nil
}

func subsequentCloseWarn(name string) {
	logrus.Error("subsequent attempt to close " + name)
	if logrus.GetLevel() >= logrus.DebugLevel {
		logrus.Errorf("stack trace: %s", string(debug.Stack()))
	}
}
