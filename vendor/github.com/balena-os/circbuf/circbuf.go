package circbuf

import "fmt"

// Buffer is a circular buffer. It has a fixed size, and new writes overwrite
// older data, such that for a buffer of size N, for any amount of writes, only
// the last N bytes are retained.
type Buffer interface {
	// Write writes up to len(buf) bytes to the internal ring, overriding older
	// data if necessary. Returns the number of bytes written and any occasional
	// error.
	Write(buf []byte) (int, error)

	// WriteByte writes a single byte into the buffer.
	WriteByte(c byte) error

	// Size returns the size of the buffer
	Size() int64

	// TotalWritten provides the total number of bytes written.
	TotalWritten() int64

	// Bytes provides a slice of the bytes written. This
	// slice should not be written to. The underlying array
	// may point to data that will be overwritten by a subsequent
	// call to Bytes. It shall do no allocation.
	Bytes() []byte

	// Get returns a single byte out of the buffer, at the given position.
	Get(i int64) (byte, error)

	// Reset resets the buffer so it has no content.
	Reset()

	// String returns the contents of the buffer as a string.
	String() string
}

// NewBuffer creates a new buffer of a given size. The size
// must be greater than 0.
func NewBuffer(size int64) (Buffer, error) {
	if size <= 0 {
		return nil, fmt.Errorf("Size must be positive")
	}

	if (size & (size - 1)) == 0 {
		b := &po2Buffer{
			baseBuffer{
				size: size,
				data: make([]byte, size),
				out:  make([]byte, size),
			},
		}
		return b, nil
	}

	b := &anyBuffer{
		baseBuffer{
			size: size,
			data: make([]byte, size),
			out:  make([]byte, size),
		},
	}
	return b, nil

}
