package ioutils

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
)

type readCloserWrapper struct {
	io.Reader
	closer func() error
}

func (r *readCloserWrapper) Close() error {
	return r.closer()
}

// NewReadCloserWrapper returns a new io.ReadCloser.
func NewReadCloserWrapper(r io.Reader, closer func() error) io.ReadCloser {
	return &readCloserWrapper{
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

// bufReader allows the underlying reader to continue to produce
// output by pre-emptively reading from the wrapped reader.
// This is achieved by buffering this data in bufReader's
// expanding buffer.
type bufReader struct {
	sync.Mutex
	buf      io.ReadWriter
	reader   io.Reader
	err      error
	wait     sync.Cond
	drainBuf []byte
}

// NewBufReader returns a new bufReader.
func NewBufReader(r io.Reader) io.ReadCloser {
	reader := &bufReader{
		buf:      NewBytesPipe(nil),
		reader:   r,
		drainBuf: make([]byte, 1024),
	}
	reader.wait.L = &reader.Mutex
	go reader.drain()
	return reader
}

// NewBufReaderWithDrainbufAndBuffer returns a BufReader with drainBuffer and buffer.
func NewBufReaderWithDrainbufAndBuffer(r io.Reader, drainBuffer []byte, buffer io.ReadWriter) io.ReadCloser {
	reader := &bufReader{
		buf:      buffer,
		drainBuf: drainBuffer,
		reader:   r,
	}
	reader.wait.L = &reader.Mutex
	go reader.drain()
	return reader
}

func (r *bufReader) drain() {
	for {
		//Call to scheduler is made to yield from this goroutine.
		//This avoids goroutine looping here when n=0,err=nil, fixes code hangs when run with GCC Go.
		callSchedulerIfNecessary()
		n, err := r.reader.Read(r.drainBuf)
		r.Lock()
		if err != nil {
			r.err = err
		} else {
			if n == 0 {
				// nothing written, no need to signal
				r.Unlock()
				continue
			}
			r.buf.Write(r.drainBuf[:n])
		}
		r.wait.Signal()
		r.Unlock()
		if err != nil {
			break
		}
	}
}

func (r *bufReader) Read(p []byte) (n int, err error) {
	r.Lock()
	defer r.Unlock()
	for {
		n, err = r.buf.Read(p)
		if n > 0 {
			return n, err
		}
		if r.err != nil {
			return 0, r.err
		}
		r.wait.Wait()
	}
}

// Close closes the bufReader
func (r *bufReader) Close() error {
	closer, ok := r.reader.(io.ReadCloser)
	if !ok {
		return nil
	}
	return closer.Close()
}

// HashData returns the sha256 sum of src.
func HashData(src io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, src); err != nil {
		return "", err
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// OnEOFReader wraps a io.ReadCloser and a function
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
