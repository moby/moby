// Package pools provides a collection of pools which provide various
// data types with buffers. These can be used to lower the number of
// memory allocations and reuse buffers.
//
// New pools should be added to this package to allow them to be
// shared across packages.
//
// Utility functions which operate on pools should be added to this
// package to allow them to be reused.
package pools // import "github.com/moby/moby/pkg/pools"

import (
	"bufio"
	"io"
	"sync"

	"github.com/moby/moby/pkg/ioutils"
)

const buffer32K = 32 * 1024

var (
	// BufioReader32KPool is a pool which returns bufio.Reader with a 32K buffer.
	BufioReader32KPool = newBufioReaderPoolWithSize(buffer32K)
	// BufioWriter32KPool is a pool which returns bufio.Writer with a 32K buffer.
	BufioWriter32KPool = newBufioWriterPoolWithSize(buffer32K)
	buffer32KPool      = newBufferPoolWithSize(buffer32K)
)

// BufioReaderPool is a bufio reader that uses sync.Pool.
type BufioReaderPool struct {
	pool sync.Pool
}

// newBufioReaderPoolWithSize is unexported because new pools should be
// added here to be shared where required.
func newBufioReaderPoolWithSize(size int) *BufioReaderPool {
	return &BufioReaderPool{
		pool: sync.Pool{
			New: func() interface{} { return bufio.NewReaderSize(nil, size) },
		},
	}
}

// Get returns a bufio.Reader which reads from r. The buffer size is that of the pool.
func (bufPool *BufioReaderPool) Get(r io.Reader) *bufio.Reader {
	buf := bufPool.pool.Get().(*bufio.Reader)
	buf.Reset(r)
	return buf
}

// Put puts the bufio.Reader back into the pool.
func (bufPool *BufioReaderPool) Put(b *bufio.Reader) {
	b.Reset(nil)
	bufPool.pool.Put(b)
}

type bufferPool struct {
	pool sync.Pool
}

func newBufferPoolWithSize(size int) *bufferPool {
	return &bufferPool{
		pool: sync.Pool{
			New: func() interface{} { s := make([]byte, size); return &s },
		},
	}
}

func (bp *bufferPool) Get() *[]byte {
	return bp.pool.Get().(*[]byte)
}

func (bp *bufferPool) Put(b *[]byte) {
	bp.pool.Put(b)
}

// Copy is a convenience wrapper which uses a buffer to avoid allocation in io.Copy.
func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := buffer32KPool.Get()
	written, err = io.CopyBuffer(dst, src, *buf)
	buffer32KPool.Put(buf)
	return
}

// NewReadCloserWrapper returns a wrapper which puts the bufio.Reader back
// into the pool and closes the reader if it's an io.ReadCloser.
func (bufPool *BufioReaderPool) NewReadCloserWrapper(buf *bufio.Reader, r io.Reader) io.ReadCloser {
	return ioutils.NewReadCloserWrapper(r, func() error {
		if readCloser, ok := r.(io.ReadCloser); ok {
			readCloser.Close()
		}
		bufPool.Put(buf)
		return nil
	})
}

// BufioWriterPool is a bufio writer that uses sync.Pool.
type BufioWriterPool struct {
	pool sync.Pool
}

// newBufioWriterPoolWithSize is unexported because new pools should be
// added here to be shared where required.
func newBufioWriterPoolWithSize(size int) *BufioWriterPool {
	return &BufioWriterPool{
		pool: sync.Pool{
			New: func() interface{} { return bufio.NewWriterSize(nil, size) },
		},
	}
}

// Get returns a bufio.Writer which writes to w. The buffer size is that of the pool.
func (bufPool *BufioWriterPool) Get(w io.Writer) *bufio.Writer {
	buf := bufPool.pool.Get().(*bufio.Writer)
	buf.Reset(w)
	return buf
}

// Put puts the bufio.Writer back into the pool.
func (bufPool *BufioWriterPool) Put(b *bufio.Writer) {
	b.Reset(nil)
	bufPool.pool.Put(b)
}

// NewWriteCloserWrapper returns a wrapper which puts the bufio.Writer back
// into the pool and closes the writer if it's an io.Writecloser.
func (bufPool *BufioWriterPool) NewWriteCloserWrapper(buf *bufio.Writer, w io.Writer) io.WriteCloser {
	return ioutils.NewWriteCloserWrapper(w, func() error {
		buf.Flush()
		if writeCloser, ok := w.(io.WriteCloser); ok {
			writeCloser.Close()
		}
		bufPool.Put(buf)
		return nil
	})
}
