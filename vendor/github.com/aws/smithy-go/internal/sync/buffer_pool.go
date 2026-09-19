package sync

import (
	"bytes"
	"io"
	"sync"
)

// maxBufferSize is the largest buffer that will be returned to the pool.
//
// A pooled buffer retains the capacity its largest occupant forced it to grow
// to, for the life of the pool. Dropping oversized buffers bounds that
// retention, at the cost of re-growing after an unusually large response.
//
// We arbitrarily choose 64K.
const maxBufferSize = 64 * 1024

func newBuffer() any {
	return new(bytes.Buffer)
}

// BufferPool pools bytes.Buffers for reading response bodies during protocol
// deserialization.
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool returns a buffer pool ready for use.
func NewBufferPool() *BufferPool {
	return &BufferPool{
		pool: sync.Pool{New: newBuffer},
	}
}

// Get consumes the given reader into a buffer from the pool, returning that
// buffer.
func (p *BufferPool) Get(r io.Reader) (*bytes.Buffer, error) {
	buf := p.pool.Get().(*bytes.Buffer)
	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}

	return buf, nil
}

// Put returns the buffer if it is within the cap limit.
func (p *BufferPool) Put(buf *bytes.Buffer) {
	if buf.Cap() <= maxBufferSize {
		buf.Reset()
		p.pool.Put(buf)
	}
}
