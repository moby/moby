package msgp

type timer interface {
	StartTimer()
	StopTimer()
}

// EndlessReader is an io.Reader
// that loops over the same data
// endlessly. It is used for benchmarking.
type EndlessReader struct {
	tb     timer
	data   []byte
	offset int
}

// NewEndlessReader returns a new endless reader.
// Buffer b cannot be empty
func NewEndlessReader(b []byte, tb timer) *EndlessReader {
	if len(b) == 0 {
		panic("EndlessReader cannot be of zero length")
	}
	// Double until we reach 4K.
	for len(b) < 4<<10 {
		b = append(b, b...)
	}
	return &EndlessReader{tb: tb, data: b, offset: 0}
}

// Read implements io.Reader. In practice, it
// always returns (len(p), nil), although it
// fills the supplied slice while the benchmark
// timer is stopped.
func (c *EndlessReader) Read(p []byte) (int, error) {
	var n int
	l := len(p)
	m := len(c.data)
	nn := copy(p[n:], c.data[c.offset:])
	n += nn
	for n < l {
		n += copy(p[n:], c.data[:])
	}
	c.offset = (c.offset + l) % m
	return n, nil
}
