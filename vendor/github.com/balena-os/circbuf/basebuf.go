package circbuf

type baseBuffer struct {
	data        []byte
	out         []byte
	size        int64
	writeCursor int64
	written     int64
}

// Size returns the size of the buffer
func (b *baseBuffer) Size() int64 {
	return b.size
}

// TotalWritten provides the total number of bytes written
func (b *baseBuffer) TotalWritten() int64 {
	return b.written
}

// Bytes provides a slice of the bytes written. This
// slice should not be written to. The underlying array
// may point to data that will be overwritten by a subsequent
// call to Bytes. It does no allocation.
func (b *baseBuffer) Bytes() []byte {
	switch {
	case b.written >= b.size && b.writeCursor == 0:
		return b.data
	case b.written > b.size:
		copy(b.out, b.data[b.writeCursor:])
		copy(b.out[b.size-b.writeCursor:], b.data[:b.writeCursor])
		return b.out
	default:
		return b.data[:b.writeCursor]
	}
}

// Reset resets the buffer so it has no content.
func (b *baseBuffer) Reset() {
	b.writeCursor = 0
	b.written = 0
}

// String returns the contents of the buffer as a string
func (b *baseBuffer) String() string {
	return string(b.Bytes())
}

// write writes len(buf) bytes to the circular buffer and returns by how much
// the writeCursor must be incremented. (This function does not increment the
// writeCursor!)
func (b *baseBuffer) write(buf []byte) int64 {
	// Account for total bytes written
	n := len(buf)
	b.written += int64(n)

	// If the buffer is larger than ours, then we only care
	// about the last size bytes anyways
	if int64(n) > b.size {
		buf = buf[int64(n)-b.size:]
	}

	// Copy in place
	remain := b.size - b.writeCursor
	copy(b.data[b.writeCursor:], buf)
	if int64(len(buf)) > remain {
		copy(b.data, buf[remain:])
	}

	return int64(len(buf))
}
