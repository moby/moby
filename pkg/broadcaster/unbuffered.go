package broadcaster // import "github.com/docker/docker/pkg/broadcaster"

import (
	"context"
	"io"
	"sync"
)

// Unbuffered accumulates multiple io.WriteCloser by stream.
type Unbuffered struct {
	writersUpdateMu sync.Mutex

	writersWriteMu         chan struct{}
	initWritersWriteMuOnce sync.Once

	writers *[]io.WriteCloser
}

// Add adds new io.WriteCloser.
func (w *Unbuffered) Add(writer io.WriteCloser) {
	w.lockWritersWriteMu()
	defer w.unlockWritersWriteMu()

	w.writersUpdateMu.Lock()
	defer w.writersUpdateMu.Unlock()

	if w.writers == nil {
		w.writers = &[]io.WriteCloser{}
	}
	*w.writers = append(*w.writers, writer)
}

// Write writes bytes to all writers. Failed writers will be evicted during
// this call.
func (w *Unbuffered) Write(p []byte) (n int, err error) {
	w.lockWritersWriteMu()
	defer w.unlockWritersWriteMu()

	w.writersUpdateMu.Lock()

	// make a copy of w.writers. Even if .CleanContext sets w.writers, this
	// Write call will still use it's valid copy
	writers := w.writers

	// release w.writersUpdateMu. This allows CleanContext to set w.writers
	// to nil - but this Write call won't be affected by this, as it uses a copy
	// of that pointer. We will also be able to safely iterate over *writers:
	// clean methods never resize the slice (they only may set pointer to nil),
	// and Add and Write require w.writersWriteMu, which we are still holding
	w.writersUpdateMu.Unlock()

	if writers == nil {
		return
	}

	var evict []int
	for i, sw := range *writers {
		if n, err := sw.Write(p); err != nil || n != len(p) {
			// On error, evict the writer
			evict = append(evict, i)
		}
	}

	w.writersUpdateMu.Lock()
	// at this point w.writers might have already been set to nil, but we're
	// not affected by this as we are using a copy
	for n, i := range evict {
		*writers = append((*writers)[:i-n], (*writers)[i-n+1:]...)
	}
	w.writersUpdateMu.Unlock()

	return len(p), nil
}

func (w *Unbuffered) cleanUnlocked() {
	if w.writers == nil {
		return
	}
	for _, sw := range *w.writers {
		sw.Close()
	}
	w.writers = nil
}

func (w *Unbuffered) cleanWithWriteLock() {
	w.writersUpdateMu.Lock()
	defer w.writersUpdateMu.Unlock()

	w.cleanUnlocked()
}

// Clean closes and removes all writers. Last non-eol-terminated part of data
// will be saved.
func (w *Unbuffered) Clean() error {
	w.lockWritersWriteMu()
	defer w.unlockWritersWriteMu()

	w.cleanWithWriteLock()
	return nil
}

// CleanContext closes and removes all writers.
// CleanContext supports timeouts via the context to unblock and forcefully
// close the io streams. This function should only be used if all writers
// added to Unbuffered support concurrent calls to Close and Write: it will
// call Close while Write may be in progress in order to forcefully close
// writers
func (w *Unbuffered) CleanContext(ctx context.Context) error {
	writersWriteMu := w.getWritersWriteMu()

	select {
	case writersWriteMu <- struct{}{}:
		defer w.unlockWritersWriteMu()
	case <-ctx.Done():
		// forceful cleanup - we will call w.cleanWithWriteLock() without
		// actually holding w.writersWriteMu. This may call .Close() on a
		// WriteCloser which is being blocked insite Write call
	}

	w.cleanWithWriteLock()
	return ctx.Err()
}

func (w *Unbuffered) initWritersWriteMu() {
	w.writersWriteMu = make(chan struct{}, 1)
}

func (w *Unbuffered) getWritersWriteMu() chan struct{} {
	w.initWritersWriteMuOnce.Do(w.initWritersWriteMu)
	return w.writersWriteMu
}

func (w *Unbuffered) lockWritersWriteMu() {
	w.getWritersWriteMu() <- struct{}{}
}

func (w *Unbuffered) unlockWritersWriteMu() {
	// this is never called before 'getWritersWriteMu()', so w.writersWriteMu is
	// guaranteed to be initialized
	<-w.writersWriteMu
}
