// Copyright 2016 the Go-FUSE Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package fuse

import (
	"os"
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
// slices with capacity of a multiple of page size, which have possibly
// been used, and may contain random contents. When using
// NewBufferPool, file system handlers may not hang on to passed-in
// buffers beyond the handler's return.
func NewBufferPool() BufferPool {
	bp := new(bufferPoolImpl)
	return bp
}

var pageSize = os.Getpagesize()

func (p *bufferPoolImpl) getPool(pageCount int) *sync.Pool {
	p.lock.Lock()
	for len(p.buffersBySize) < pageCount+1 {
		p.buffersBySize = append(p.buffersBySize, nil)
	}
	if p.buffersBySize[pageCount] == nil {
		p.buffersBySize[pageCount] = &sync.Pool{
			New: func() interface{} { return make([]byte, pageSize*pageCount) },
		}
	}
	pool := p.buffersBySize[pageCount]
	p.lock.Unlock()
	return pool
}

func (p *bufferPoolImpl) AllocBuffer(size uint32) []byte {
	sz := int(size)
	if sz < pageSize {
		sz = pageSize
	}

	if sz%pageSize != 0 {
		sz += pageSize
	}
	pages := sz / pageSize

	b := p.getPool(pages).Get().([]byte)
	return b[:size]
}

func (p *bufferPoolImpl) FreeBuffer(slice []byte) {
	if slice == nil {
		return
	}
	if cap(slice)%pageSize != 0 || cap(slice) == 0 {
		return
	}
	pages := cap(slice) / pageSize
	slice = slice[:cap(slice)]

	p.getPool(pages).Put(slice)
}
