package writeflusher

import (
	"io"
	"net/http"
	"sync"
)

type flusher interface {
	Flush()
}

var errWriteFlusherClosed = io.EOF

// NopFlusher represents a type which flush operation is nop.
type NopFlusher struct{}

// Flush is a nop operation.
func (f *NopFlusher) Flush() {}

// WriteFlusher wraps the Write and Flush operation ensuring that every write
// is a flush. In addition, the Close method can be called to intercept
// Read/Write calls if the targets lifecycle has already ended.
type WriteFlusher struct {
	w          io.Writer
	flusher    flusher
	closed     chan struct{}
	closeLock  sync.Mutex
	firstFlush sync.Once
}

// NewWriteFlusher returns a new WriteFlusher.
func NewWriteFlusher(w io.Writer) *WriteFlusher {
	var fl flusher
	if f, ok := w.(flusher); ok {
		fl = f
	} else {
		fl = &NopFlusher{}
	}
	return &WriteFlusher{w: w, flusher: fl, closed: make(chan struct{})}
}

func (wf *WriteFlusher) Write(b []byte) (n int, err error) {
	select {
	case <-wf.closed:
		return 0, errWriteFlusherClosed
	default:
	}

	n, err = wf.w.Write(b)
	wf.Flush() // every write is a flush.
	return n, err
}

// Flush the stream immediately.
func (wf *WriteFlusher) Flush() {
	select {
	case <-wf.closed:
		return
	default:
	}

	// Here we call WriteHeader() if the io.Writer is an http.ResponseWriter to ensure that we don't try
	// to send headers multiple times if the writer has already been wrapped by OTEL instrumentation
	// (which doesn't wrap the Flush() func. See https://github.com/moby/moby/issues/47448)
	wf.firstFlush.Do(func() {
		if rw, ok := wf.w.(http.ResponseWriter); ok {
			rw.WriteHeader(http.StatusOK)
		}
	})

	wf.flusher.Flush()
}

// Close closes the write flusher, disallowing any further writes to the
// target. After the flusher is closed, all calls to write or flush will
// result in an error.
func (wf *WriteFlusher) Close() error {
	wf.closeLock.Lock()
	defer wf.closeLock.Unlock()

	select {
	case <-wf.closed:
		return errWriteFlusherClosed
	default:
		close(wf.closed)
	}
	return nil
}
