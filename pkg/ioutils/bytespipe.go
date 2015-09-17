package ioutils

const maxCap = 1e6

// BytesPipe is io.ReadWriter which works similarly to pipe(queue).
// All written data could be read only once. Also BytesPipe is allocating
// and releasing new byte slices to adjust to current needs, so there won't be
// overgrown buffer after high load peak.
// BytesPipe isn't goroutine-safe, caller must synchronize it if needed.
type BytesPipe struct {
	buf      [][]byte // slice of byte-slices of buffered data
	lastRead int      // index in the first slice to a read point
	bufLen   int      // length of data buffered over the slices
}

// NewBytesPipe creates new BytesPipe, initialized by specified slice.
// If buf is nil, then it will be initialized with slice which cap is 64.
// buf will be adjusted in a way that len(buf) == 0, cap(buf) == cap(buf).
func NewBytesPipe(buf []byte) *BytesPipe {
	if cap(buf) == 0 {
		buf = make([]byte, 0, 64)
	}
	return &BytesPipe{
		buf: [][]byte{buf[:0]},
	}
}

// Write writes p to BytesPipe.
// It can allocate new []byte slices in a process of writing.
func (bp *BytesPipe) Write(p []byte) (n int, err error) {
	for {
		// write data to the last buffer
		b := bp.buf[len(bp.buf)-1]
		// copy data to the current empty allocated area
		n := copy(b[len(b):cap(b)], p)
		// increment buffered data length
		bp.bufLen += n
		// include written data in last buffer
		bp.buf[len(bp.buf)-1] = b[:len(b)+n]

		// if there was enough room to write all then break
		if len(p) == n {
			break
		}

		// more data: write to the next slice
		p = p[n:]
		// allocate slice that has twice the size of the last unless maximum reached
		nextCap := 2 * cap(bp.buf[len(bp.buf)-1])
		if maxCap < nextCap {
			nextCap = maxCap
		}
		// add new byte slice to the buffers slice and continue writing
		bp.buf = append(bp.buf, make([]byte, 0, nextCap))
	}
	return
}

func (bp *BytesPipe) len() int {
	return bp.bufLen - bp.lastRead
}

// Read reads bytes from BytesPipe.
// Data could be read only once.
func (bp *BytesPipe) Read(p []byte) (n int, err error) {
	for {
		read := copy(p, bp.buf[0][bp.lastRead:])
		n += read
		bp.lastRead += read
		if bp.len() == 0 {
			// we have read everything. reset to the beginning.
			bp.lastRead = 0
			bp.bufLen -= len(bp.buf[0])
			bp.buf[0] = bp.buf[0][:0]
			break
		}
		// break if everything was read
		if len(p) == read {
			break
		}
		// more buffered data and more asked. read from next slice.
		p = p[read:]
		bp.lastRead = 0
		bp.bufLen -= len(bp.buf[0])
		bp.buf[0] = nil     // throw away old slice
		bp.buf = bp.buf[1:] // switch to next
	}
	return
}
