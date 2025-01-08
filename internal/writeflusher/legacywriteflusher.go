package writeflusher

import (
	"io"
	"sync"
)

// Deprecated: use the internal WriteFlusher instead.
// This is the old implementation that lived in ioutils.

// This struct and all funcs below used to live in the pkg/ioutils package
//
// LegacyWriteFlusher wraps the Write and Flush operation ensuring that every write
// is a flush. In addition, the Close method can be called to intercept
// Read/Write calls if the targets lifecycle has already ended.
//
// Deprecated: use the internal writeflusher.WriteFlusher instead
type LegacyWriteFlusher struct {
	w           io.Writer
	flusher     flusher
	flushed     chan struct{}
	flushedOnce sync.Once
	closed      chan struct{}
	closeLock   sync.Mutex
}

// NewLegacyWriteFlusher returns a new LegacyWriteFlusher.
//
// Deprecated: use the internal writeflusher.NewWriteFlusher() instead
func NewLegacyWriteFlusher(w io.Writer) *LegacyWriteFlusher {
	var fl flusher
	if f, ok := w.(flusher); ok {
		fl = f
	} else {
		fl = &NopFlusher{}
	}
	return &LegacyWriteFlusher{w: w, flusher: fl, closed: make(chan struct{}), flushed: make(chan struct{})}
}

// Deprecated: use the internal writeflusher.Write() instead
func (wf *LegacyWriteFlusher) Write(b []byte) (n int, err error) {
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
//
// Deprecated: use the internal writeflusher.Flush() instead
func (wf *LegacyWriteFlusher) Flush() {
	select {
	case <-wf.closed:
		return
	default:
	}

	wf.flushedOnce.Do(func() {
		close(wf.flushed)
	})
	wf.flusher.Flush()
}

// Flushed returns the state of flushed.
// If it's flushed, return true, or else it return false.
//
// Deprecated: use the internal writeflusher.WriteFlusher instead
func (wf *LegacyWriteFlusher) Flushed() bool {
	// BUG(stevvooe): Remove this method. Its use is inherently racy. Seems to
	// be used to detect whether or a response code has been issued or not.
	// Another hook should be used instead.
	var flushed bool
	select {
	case <-wf.flushed:
		flushed = true
	default:
	}
	return flushed
}

// Close closes the write flusher, disallowing any further writes to the
// target. After the flusher is closed, all calls to write or flush will
// result in an error.
//
// Deprecated: use the internal writeflusher.Close() instead
func (wf *LegacyWriteFlusher) Close() error {
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
