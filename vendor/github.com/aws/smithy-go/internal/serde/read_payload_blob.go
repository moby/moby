package serde

import (
	"bytes"
	"io"
)

// maxPresize bounds how large a buffer we will allocate up front from a
// Content-Length header. Past this point we allocate maxPresize and let the read
// grow the buffer to fit whatever actually arrives.
//
// We arbitrarily choose 512K.
const maxPresize = 512 * 1024

// ReadPayloadBlob consumes the given reader into a buffer that does not come
// from (and will never be returned to) a pool, sizing it from contentLength
// when that is known and within bounds.
func ReadPayloadBlob(r io.Reader, contentLength int64) (*bytes.Buffer, error) {
	buf := &bytes.Buffer{}
	if contentLength > 0 {
		presize := contentLength
		if presize > maxPresize {
			presize = maxPresize
		}

		// + bytes.MinRead prevents pointless doubling on EOF
		buf.Grow(int(presize) + bytes.MinRead)
	}

	if _, err := buf.ReadFrom(r); err != nil {
		return nil, err
	}
	return buf, nil
}
