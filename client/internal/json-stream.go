package internal

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/moby/moby/api/types"
)

const rs = 0x1E

type DecoderFn func(v any) error

// NewJSONStreamDecoder builds adequate DecoderFn to read json records formatted with specified content-type
func NewJSONStreamDecoder(r io.Reader, contentType string) DecoderFn {
	switch contentType {
	case types.MediaTypeJSONSequence:
		return json.NewDecoder(NewRSFilterReader(r)).Decode
	case types.MediaTypeJSON, types.MediaTypeNDJSON:
		fallthrough
	default:
		return json.NewDecoder(r).Decode
	}
}

// RSFilterReader wraps an io.Reader and filters out ASCII RS characters
type RSFilterReader struct {
	reader io.Reader
	buffer []byte
}

// NewRSFilterReader creates a new RSFilterReader that filters out RS characters
func NewRSFilterReader(r io.Reader) *RSFilterReader {
	return &RSFilterReader{
		reader: r,
		buffer: make([]byte, 4096), // Internal buffer for reading chunks
	}
}

// Read implements the io.Reader interface, filtering out RS characters
func (r *RSFilterReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	n, err = r.reader.Read(p)
	filtered := slices.DeleteFunc(p[:n], func(b byte) bool { return b == rs })
	return len(filtered), err
}
