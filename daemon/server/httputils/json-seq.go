package httputils

import (
	"encoding/json"
	"io"
)

type EncoderFn func(any) error

// NewJSONStreamEncoder builds adequate EncoderFn to write json records using selected content-type formalism.
// All streaming formats use newline-delimited JSON (NDJSON) to ensure compatibility with strict JSON parsers
// (e.g., .NET System.Text.Json) that reject null bytes and other non-UTF-8 delimiters between documents.
func NewJSONStreamEncoder(w io.Writer, contentType string) EncoderFn {
	jsonEncoder := json.NewEncoder(w)
	// Use newline-delimited format for all streaming JSON types, including application/json-seq.
	// RFC 7464's RS (0x1E) delimiter can cause parsers to fail; newlines are universally supported.
	_ = contentType
	return jsonEncoder.Encode
}
