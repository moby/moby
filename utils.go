package docker

import (
	"bytes"
	"container/list"
	"io"
	"sync"
)

type bufReader struct {
	buf    *bytes.Buffer
	reader io.Reader
	err    error
	l      sync.Mutex
	wait   sync.Cond
}

func newBufReader(r io.Reader) *bufReader {
	reader := &bufReader{
		buf:    &bytes.Buffer{},
		reader: r,
	}
	reader.wait.L = &reader.l
	go reader.drain()
	return reader
}

func (r *bufReader) drain() {
	buf := make([]byte, 1024)
	for {
		n, err := r.reader.Read(buf)
		if err != nil {
			r.err = err
		} else {
			r.buf.Write(buf[0:n])
		}
		r.l.Lock()
		r.wait.Signal()
		r.l.Unlock()
		if err != nil {
			break
		}
	}
}

func (r *bufReader) Read(p []byte) (n int, err error) {
	for {
		n, err = r.buf.Read(p)
		if n > 0 {
			return n, err
		}
		if r.err != nil {
			return 0, r.err
		}
		r.l.Lock()
		r.wait.Wait()
		r.l.Unlock()
	}
	return
}

func (r *bufReader) Close() error {
	closer, ok := r.reader.(io.ReadCloser)
	if !ok {
		return nil
	}
	return closer.Close()
}

type writeBroadcaster struct {
	writers *list.List
}

func (w *writeBroadcaster) AddWriter(writer io.WriteCloser) {
	w.writers.PushBack(writer)
}

func (w *writeBroadcaster) RemoveWriter(writer io.WriteCloser) {
	for e := w.writers.Front(); e != nil; e = e.Next() {
		v := e.Value.(io.Writer)
		if v == writer {
			w.writers.Remove(e)
			return
		}
	}
}

func (w *writeBroadcaster) Write(p []byte) (n int, err error) {
	failed := []*list.Element{}
	for e := w.writers.Front(); e != nil; e = e.Next() {
		writer := e.Value.(io.Writer)
		if n, err := writer.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			failed = append(failed, e)
		}
	}
	// We cannot remove while iterating, so it has to be done in
	// a separate step
	for _, e := range failed {
		w.writers.Remove(e)
	}
	return len(p), nil
}

func (w *writeBroadcaster) Close() error {
	for e := w.writers.Front(); e != nil; e = e.Next() {
		writer := e.Value.(io.WriteCloser)
		writer.Close()
	}
	return nil
}

func newWriteBroadcaster() *writeBroadcaster {
	return &writeBroadcaster{list.New()}
}
