package internal

import (
	"encoding/json"
	"io"
	"strings"

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

	for n == 0 { // Keep reading until we have some filtered data or hit EOF/error
		// Read from underlying reader
		readN, readErr := r.reader.Read(r.buffer)
		if readErr != nil && readErr != io.EOF {
			return 0, readErr
		}

		if readN == 0 {
			return 0, readErr // Will be io.EOF
		}

		// Filter out RS characters and copy to output buffer
		writePos := 0
		for i := 0; i < readN; i++ {
			if r.buffer[i] != rs { // Skip RS characters
				if writePos < len(p) {
					p[writePos] = r.buffer[i]
					writePos++
				} else {
					// Output buffer is full, we need to handle remaining data
					// Create a temporary reader for the remaining filtered data
					remaining := make([]byte, 0, readN-i)
					for j := i; j < readN; j++ {
						if r.buffer[j] != rs {
							remaining = append(remaining, r.buffer[j])
						}
					}

					// Create a new reader that combines remaining data with original reader
					if len(remaining) > 0 {
						r.reader = io.MultiReader(strings.NewReader(string(remaining)), r.reader)
					}
					n = writePos
					return n, readErr
				}
			}
		}
		n = writePos

		// If we hit EOF and have data, return it
		if readErr == io.EOF && n > 0 {
			return n, nil
		}
		// If we hit EOF and have no data, return EOF
		if readErr == io.EOF {
			return 0, io.EOF
		}
	}

	return n, err
}
