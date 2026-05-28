package internal

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/moby/moby/api/types"
)

const rs = 0x1E

type DecoderFn func(v any) error

// NewJSONStreamDecoder builds a DecoderFn to read a stream of JSON records
// formatted with the specified content-type.
func NewJSONStreamDecoder(r io.Reader, contentType types.MediaType) DecoderFn {
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

func (r *rsFilterReader) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	for {
		n, err := r.reader.Read(p)
		if n == 0 {
			return 0, err
		}

		filtered := slices.DeleteFunc(p[:n], func(b byte) bool { return b == rs })
		n = len(filtered)
		if err != nil {
			if err == io.EOF && n > 0 {
				return n, nil
			}
			return n, err
		}
		if n == 0 {
			// Avoid returning (0, nil) after consuming input; keep reading until data or an error (e.g., EOF).
			continue
		}
		return n, nil
	}
}
