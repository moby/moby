package ioutils

import (
	"io"
	"sync"
)

const bufSize = 32 * 1024

var bufPool = &sync.Pool{
	New: func() interface{} { return make([]byte, bufSize) },
}

// Copy calls io.CopyBuffer with buffer from sync.Pool.
// Buffer size is 32K.
func Copy(dst io.Writer, src io.Reader) (written int64, err error) {
	buf := bufPool.Get().([]byte)
	written, err = io.CopyBuffer(dst, src, buf)
	bufPool.Put(buf)
	return
}
