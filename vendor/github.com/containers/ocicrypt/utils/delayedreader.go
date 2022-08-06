/*
   Copyright The ocicrypt Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package utils

import (
	"io"
)

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// DelayedReader wraps a io.Reader and allows a client to use the Reader
// interface. The DelayedReader holds back some buffer to the client
// so that it can report any error that occurred on the Reader it wraps
// early to the client while it may still have held some data back.
type DelayedReader struct {
	reader   io.Reader // Reader to Read() bytes from and delay them
	err      error     // error that occurred on the reader
	buffer   []byte    // delay buffer
	bufbytes int       // number of bytes in the delay buffer to give to Read(); on '0' we return 'EOF' to caller
	bufoff   int       // offset in the delay buffer to give to Read()
}

// NewDelayedReader wraps a io.Reader and allocates a delay buffer of bufsize bytes
func NewDelayedReader(reader io.Reader, bufsize uint) io.Reader {
	return &DelayedReader{
		reader: reader,
		buffer: make([]byte, bufsize),
	}
}

// Read implements the io.Reader interface
func (dr *DelayedReader) Read(p []byte) (int, error) {
	if dr.err != nil && dr.err != io.EOF {
		return 0, dr.err
	}

	// if we are completely drained, return io.EOF
	if dr.err == io.EOF && dr.bufbytes == 0 {
		return 0, io.EOF
	}

	// only at the beginning we fill our delay buffer in an extra step
	if dr.bufbytes < len(dr.buffer) && dr.err == nil {
		dr.bufbytes, dr.err = FillBuffer(dr.reader, dr.buffer)
		if dr.err != nil && dr.err != io.EOF {
			return 0, dr.err
		}
	}
	// dr.err != nil means we have EOF and can drain the delay buffer
	// otherwise we need to still read from the reader

	var tmpbuf []byte
	tmpbufbytes := 0
	if dr.err == nil {
		tmpbuf = make([]byte, len(p))
		tmpbufbytes, dr.err = FillBuffer(dr.reader, tmpbuf)
		if dr.err != nil && dr.err != io.EOF {
			return 0, dr.err
		}
	}

	// copy out of the delay buffer into 'p'
	tocopy1 := min(len(p), dr.bufbytes)
	c1 := copy(p[:tocopy1], dr.buffer[dr.bufoff:])
	dr.bufoff += c1
	dr.bufbytes -= c1

	c2 := 0
	// can p still hold more data?
	if c1 < len(p) {
		// copy out of the tmpbuf into 'p'
		c2 = copy(p[tocopy1:], tmpbuf[:tmpbufbytes])
	}

	// if tmpbuf holds data we need to hold onto, copy them
	// into the delay buffer
	if tmpbufbytes-c2 > 0 {
		// left-shift the delay buffer and append the tmpbuf's remaining data
		dr.buffer = dr.buffer[dr.bufoff : dr.bufoff+dr.bufbytes]
		dr.buffer = append(dr.buffer, tmpbuf[c2:tmpbufbytes]...)
		dr.bufoff = 0
		dr.bufbytes = len(dr.buffer)
	}

	var err error
	if dr.bufbytes == 0 {
		err = io.EOF
	}
	return c1 + c2, err
}
