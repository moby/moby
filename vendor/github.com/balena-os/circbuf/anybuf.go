package circbuf

import (
	"fmt"
)

// anyBuffer implements a circular buffer of any size.
type anyBuffer struct {
	baseBuffer
}

// Write writes up to len(buf) bytes to the internal ring,
// overriding older data if necessary.
func (b *anyBuffer) Write(buf []byte) (int, error) {
	n := b.write(buf)
	b.writeCursor = ((b.writeCursor + n) % b.size)
	return len(buf), nil
}

// WriteByte writes a single byte into the buffer.
func (b *anyBuffer) WriteByte(c byte) error {
	b.data[b.writeCursor] = c
	b.writeCursor = ((b.writeCursor + 1) % b.size)
	b.written++
	return nil
}

// Get returns a single byte out of the buffer, at the given position.
func (b *anyBuffer) Get(i int64) (byte, error) {
	switch {
	case i >= b.written || i >= b.size:
		return 0, fmt.Errorf("Index out of bounds: %v", i)
	case b.written > b.size:
		return b.data[(b.writeCursor+i)%b.size], nil
	default:
		return b.data[i], nil
	}
}
