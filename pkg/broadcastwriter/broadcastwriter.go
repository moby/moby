package broadcastwriter

import (
	"io"
	"sync"
)

// BroadcastWriter accumulate multiple io.WriteCloser by stream.
type BroadcastWriter struct {
	sync.Mutex
	writers map[io.WriteCloser]struct{}
}

// AddWriter adds new io.WriteCloser.
func (w *BroadcastWriter) AddWriter(writer io.WriteCloser) {
	w.Lock()
	w.writers[writer] = struct{}{}
	w.Unlock()
}

// Write writes bytes to all writers. Failed writers will be evicted during
// this call.
func (w *BroadcastWriter) Write(p []byte) (n int, err error) {
	w.Lock()
	for sw := range w.writers {
		if n, err := sw.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			delete(w.writers, sw)
		}
	}
	w.Unlock()
	return len(p), nil
}

// Clean closes and removes all writers. Last non-eol-terminated part of data
// will be saved.
func (w *BroadcastWriter) Clean() error {
	w.Lock()
	for w := range w.writers {
		w.Close()
	}
	w.writers = make(map[io.WriteCloser]struct{})
	w.Unlock()
	return nil
}

func New() *BroadcastWriter {
	return &BroadcastWriter{
		writers: make(map[io.WriteCloser]struct{}),
	}
}
