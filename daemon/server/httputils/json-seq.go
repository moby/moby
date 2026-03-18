package httputils

import (
	"encoding/json"
	"io"

	"github.com/moby/moby/api/types"
)

const rs = 0x1E

type EncoderFn func(any) error

// NewJSONStreamEncoder builds adequate EncoderFn to write json records using selected content-type formalism
func NewJSONStreamEncoder(w io.Writer, contentType string) EncoderFn {
	jsonEncoder := json.NewEncoder(w)
	switch contentType {
	case types.MediaTypeJSONSequence:
		jseq := &jsonSeq{
			w:    w,
			json: jsonEncoder,
		}
		return jseq.Encode
	case types.MediaTypeNDJSON, types.MediaTypeJSON, types.MediaTypeJSONLines:
		fallthrough
	default:
		return jsonEncoder.Encode
	}
}

type jsonSeq struct {
	w    io.Writer
	json *json.Encoder
}

// Encode prefixes every written record with an ASCII record separator.
func (js *jsonSeq) Encode(record any) error {
	_, err := js.w.Write([]byte{rs})
	if err != nil {
		return err
	}
	// JSON-seq also requires a LF character, bu json.Encoder already adds one
	return js.json.Encode(record)
}
