package broadcastwriter

import (
	"io"
	"sync"
)

// BroadcastWriter accumulate multiple io.WriteCloser by stream.
type BroadcastWriter struct {
	mu      sync.Mutex
	streams map[io.WriteCloser]struct{}
}

// AddWriter adds new io.WriteCloser,
func (w *BroadcastWriter) AddWriter(writer io.WriteCloser) {
	w.mu.Lock()
	w.streams[writer] = struct{}{}
	w.mu.Unlock()
}

// Write writes bytes to all writers. Failed writers will be evicted during
// this call.
func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	for sw := range w.streams {
		if n, err := sw.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			delete(w.streams, sw)
		}
	}
	w.mu.Unlock()
	return len(p), nil
}

// Clean closes and removes all writers. Last non-eol-terminated part of data
// will be saved.
func (w *BroadcastWriter) Clean() error {
	w.mu.Lock()
	for sw := range w.streams {
		sw.Close()
	}
	w.streams = make(map[io.WriteCloser]struct{})
	w.mu.Unlock()
	return nil
}

func New() *BroadcastWriter {
	return &BroadcastWriter{
		streams: make(map[io.WriteCloser]struct{}),
	}
}
