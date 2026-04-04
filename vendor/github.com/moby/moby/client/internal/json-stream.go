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
	case types.MediaTypeJSON, types.MediaTypeNDJSON, types.MediaTypeJSONLines:
		fallthrough
	default:
		return json.NewDecoder(r).Decode
	}
}

type rsFilterReader struct {
	reader io.Reader
}

// NewRSFilterReader creates an [io.Reader] that filters out ASCII Record Separators (RS).
func NewRSFilterReader(r io.Reader) io.Reader {
	return &rsFilterReader{reader: r}
}

func (r *rsFilterReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}

	n, err = r.reader.Read(p)
	filtered := slices.DeleteFunc(p[:n], func(b byte) bool { return b == rs })
	return len(filtered), err
}
