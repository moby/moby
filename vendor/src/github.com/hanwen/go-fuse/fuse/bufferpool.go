package fuse

import (
	"sync"
)

var paranoia bool

// BufferPool implements explicit memory management. It is used for
// minimizing the GC overhead of communicating with the kernel.
type BufferPool interface {
	// AllocBuffer creates a buffer of at least the given size. After use,
	// it should be deallocated with FreeBuffer().
	AllocBuffer(size uint32) []byte

	// FreeBuffer takes back a buffer if it was allocated through
	// AllocBuffer.  It is not an error to call FreeBuffer() on a slice
	// obtained elsewhere.
	FreeBuffer(slice []byte)
}

type gcBufferPool struct {
}

// NewGcBufferPool is a fallback to the standard allocation routines.
func NewGcBufferPool() BufferPool {
	return &gcBufferPool{}
}

func (p *gcBufferPool) AllocBuffer(size uint32) []byte {
	return make([]byte, size)
}

func (p *gcBufferPool) FreeBuffer(slice []byte) {
}

type bufferPoolImpl struct {
	lock sync.Mutex

	// For each page size multiple a list of slice pointers.
	buffersBySize []*sync.Pool
}

// NewBufferPool returns a BufferPool implementation that that returns
// slices with capacity of a multiple of PAGESIZE, which have possibly
// been used, and may contain random contents. When using
// NewBufferPool, file system handlers may not hang on to passed-in
// buffers beyond the handler's return.
func NewBufferPool() BufferPool {
	bp := new(bufferPoolImpl)
	return bp
}

func (p *bufferPoolImpl) getPool(pageCount int) *sync.Pool {
	p.lock.Lock()
	for len(p.buffersBySize) < pageCount+1 {
		p.buffersBySize = append(p.buffersBySize, nil)
	}
	if p.buffersBySize[pageCount] == nil {
		p.buffersBySize[pageCount] = &sync.Pool{
			New: func() interface{} { return make([]byte, PAGESIZE*pageCount) },
		}
	}
	pool := p.buffersBySize[pageCount]
	p.lock.Unlock()
	return pool
}

func (p *bufferPoolImpl) AllocBuffer(size uint32) []byte {
	sz := int(size)
	if sz < PAGESIZE {
		sz = PAGESIZE
	}

	if sz%PAGESIZE != 0 {
		sz += PAGESIZE
	}
	pages := sz / PAGESIZE

	b := p.getPool(pages).Get().([]byte)
	return b[:size]
}

func (p *bufferPoolImpl) FreeBuffer(slice []byte) {
	if slice == nil {
		return
	}
	if cap(slice)%PAGESIZE != 0 || cap(slice) == 0 {
		return
	}
	pages := cap(slice) / PAGESIZE
	slice = slice[:cap(slice)]

	p.getPool(pages).Put(slice)
}
