package fsutil

import (
	"io"
)

const chunkSize = 32 * 1024

type buffer struct {
	chunks [][]byte
}

func (b *buffer) alloc(n int) []byte {
	if n > chunkSize {
		buf := make([]byte, n)
		b.chunks = append(b.chunks, buf)
		return buf
	}

	if len(b.chunks) != 0 {
		lastChunk := b.chunks[len(b.chunks)-1]
		l := len(lastChunk)
		if l+n <= cap(lastChunk) {
			lastChunk = lastChunk[:l+n]
			b.chunks[len(b.chunks)-1] = lastChunk
			return lastChunk[l : l+n]
		}
	}

	buf := make([]byte, n, chunkSize)
	b.chunks = append(b.chunks, buf)
	return buf
}

func (b *buffer) WriteTo(w io.Writer) (n int64, err error) {
	for _, c := range b.chunks {
		m, err := w.Write(c)
		n += int64(m)
		if err != nil {
			return n, err
		}
	}
	return n, nil
}
