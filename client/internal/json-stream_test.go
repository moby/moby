package internal

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/moby/moby/api/types"
	"gotest.tools/v3/assert"
)

func TestJSONStreamDecode_JSONSequence(t *testing.T) {
	const separator = string(rune(rs))
	const lf = "\n"
	input := fmt.Sprintf(`%s{"hello":"world"}%s%s{ "hello": "again" }%s`, separator, lf, separator, lf)
	decoder := NewJSONStreamDecoder(strings.NewReader(input), types.MediaTypeJSONSequence)
	type Hello struct {
		Hello string `json:"hello"`
	}
	var hello Hello
	err := decoder(&hello)
	assert.NilError(t, err)
	assert.Equal(t, "world", hello.Hello)

	var again Hello
	err = decoder(&again)
	assert.NilError(t, err)
	assert.Equal(t, "again", again.Hello)
}

// TestRSFilterReader_SkipsRSOnlyRead verifies that RSFilterReader does not
// return (0, nil) when the underlying reader produces only RS bytes in a read.
// It must continue reading until it can return actual data or an error.
func TestRSFilterReader_SkipsRSOnlyRead(t *testing.T) {
	r := NewRSFilterReader(&chunkedReader{
		reads: [][]byte{
			{rs},
			[]byte(`{"hello":"world"}`),
		},
	})

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	assert.NilError(t, err)
	assert.Equal(t, `{"hello":"world"}`, string(buf[:n]))
}

// TestRSFilterReader_SkipsRSBeforeEOF verifies that RSFilterReader correctly
// skips RS-only input and propagates io.EOF when no non-RS data is available.
func TestRSFilterReader_SkipsRSBeforeEOF(t *testing.T) {
	r := NewRSFilterReader(&chunkedReader{
		reads: [][]byte{{rs}},
	})

	buf := make([]byte, 64)
	n, err := r.Read(buf)
	assert.Assert(t, err == io.EOF)
	assert.Equal(t, 0, n)
}

type chunkedReader struct {
	reads [][]byte
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if len(r.reads) == 0 {
		return 0, io.EOF
	}

	n := copy(p, r.reads[0])
	r.reads = r.reads[1:]
	return n, nil
}
