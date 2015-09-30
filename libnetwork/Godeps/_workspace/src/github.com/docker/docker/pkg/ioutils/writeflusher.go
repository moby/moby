package ioutils

import (
	"io"
	"net/http"
	"sync"
)

// WriteFlusher wraps the Write and Flush operation.
type WriteFlusher struct {
	sync.Mutex
	w       io.Writer
	flusher http.Flusher
	flushed bool
}

func (wf *WriteFlusher) Write(b []byte) (n int, err error) {
	wf.Lock()
	defer wf.Unlock()
	n, err = wf.w.Write(b)
	wf.flushed = true
	wf.flusher.Flush()
	return n, err
}

// Flush the stream immediately.
func (wf *WriteFlusher) Flush() {
	wf.Lock()
	defer wf.Unlock()
	wf.flushed = true
	wf.flusher.Flush()
}

// Flushed returns the state of flushed.
// If it's flushed, return true, or else it return false.
func (wf *WriteFlusher) Flushed() bool {
	wf.Lock()
	defer wf.Unlock()
	return wf.flushed
}

// NewWriteFlusher returns a new WriteFlusher.
func NewWriteFlusher(w io.Writer) *WriteFlusher {
	var flusher http.Flusher
	if f, ok := w.(http.Flusher); ok {
		flusher = f
	} else {
		flusher = &NopFlusher{}
	}
	return &WriteFlusher{w: w, flusher: flusher}
}
