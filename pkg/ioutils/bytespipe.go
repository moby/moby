package ioutils

const maxCap = 10 * 1e6

// BytesPipe is io.ReadWriter which works similary to pipe(queue).
// All written data could be read only once. Also BytesPipe trying to adjust
// internal []byte slice to current needs, so there won't be overgrown buffer
// after highload peak.
// BytesPipe isn't goroutine-safe, caller must synchronize it if needed.
type BytesPipe struct {
	buf      []byte
	lastRead int
}

// NewBytesPipe creates new BytesPipe, initialized by specified slice.
// If buf is nil, then it will be initialized with slice which cap is 64.
// buf will be adjusted in a way that len(buf) == 0, cap(buf) == cap(buf).
func NewBytesPipe(buf []byte) *BytesPipe {
	if cap(buf) == 0 {
		buf = make([]byte, 0, 64)
	}
	return &BytesPipe{
		buf: buf[:0],
	}
}

func (bp *BytesPipe) grow(n int) {
	if len(bp.buf)+n > cap(bp.buf) {
		// not enough space
		var buf []byte
		remain := bp.len()
		if remain+n <= cap(bp.buf)/2 {
			// enough space in current buffer, just move data to head
			copy(bp.buf, bp.buf[bp.lastRead:])
			buf = bp.buf[:remain]
		} else {
			// reallocate buffer
			buf = make([]byte, remain, 2*cap(bp.buf)+n)
			copy(buf, bp.buf[bp.lastRead:])
		}
		bp.buf = buf
		bp.lastRead = 0
	}
}

// Write writes p to BytesPipe.
// It can increase cap of internal []byte slice in a process of writing.
func (bp *BytesPipe) Write(p []byte) (n int, err error) {
	bp.grow(len(p))
	bp.buf = append(bp.buf, p...)
	return
}

func (bp *BytesPipe) len() int {
	return len(bp.buf) - bp.lastRead
}

func (bp *BytesPipe) crop() {
	// shortcut for empty buffer
	if bp.lastRead == len(bp.buf) {
		bp.lastRead = 0
		bp.buf = bp.buf[:0]
	}
	r := bp.len()
	// if we have too large buffer for too small data
	if cap(bp.buf) > maxCap && r < cap(bp.buf)/10 {
		copy(bp.buf, bp.buf[bp.lastRead:])
		// will use same underlying slice until reach cap
		bp.buf = bp.buf[:r : cap(bp.buf)/2]
		bp.lastRead = 0
	}
}

// Read reads bytes from BytesPipe.
// Data could be read only once.
// Internal []byte slice could be shrinked.
func (bp *BytesPipe) Read(p []byte) (n int, err error) {
	n = copy(p, bp.buf[bp.lastRead:])
	bp.lastRead += n
	bp.crop()
	return
}
