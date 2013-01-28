package docker

import (
	"bytes"
	"container/list"
	"io"
	"os/exec"
	"sync"
)

// Tar generates a tar archive from a filesystem path, and returns it as a stream.
// Path must point to a directory.

func Tar(path string) (io.Reader, error) {
	cmd := exec.Command("tar", "-C", path, "-c", ".")
	output, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// FIXME: errors will not be passed because we don't wait for the command.
	// Instead, consumers will hit EOF right away.
	// This can be fixed by waiting for the process to exit, or for the first write
	// on stdout, whichever comes first.
	return output, nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}

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
