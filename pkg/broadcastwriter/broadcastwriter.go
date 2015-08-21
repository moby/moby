package broadcastwriter

import (
	"io"
	"sync"
)

// BroadcastWriter accumulate multiple io.WriteCloser by stream.
type BroadcastWriter struct {
	mu      sync.Mutex
	writers []io.WriteCloser
}

// AddWriter adds new io.WriteCloser.
func (w *BroadcastWriter) AddWriter(writer io.WriteCloser) {
	w.mu.Lock()
	w.writers = append(w.writers, writer)
	w.mu.Unlock()
}

// Write writes bytes to all writers. Failed writers will be evicted during
// this call.
func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	var evict []int
	for i, sw := range w.writers {
		if n, err := sw.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			evict = append(evict, i)
		}
	}
	for n, i := range evict {
		w.writers = append(w.writers[:i-n], w.writers[i-n+1:]...)
	}
	w.mu.Unlock()
	return len(p), nil
}

// Clean closes and removes all writers. Last non-eol-terminated part of data
// will be saved.
func (w *BroadcastWriter) Clean() error {
	w.mu.Lock()
	for _, sw := range w.writers {
		sw.Close()
	}
	w.writers = nil
	w.mu.Unlock()
	return nil
}

// New creates a new BroadcastWriter.
func New() *BroadcastWriter {
	return &BroadcastWriter{}
}
