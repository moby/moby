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

package ocicrypt

import (
	"io"
)

type readerAtReader struct {
	r   io.ReaderAt
	off int64
}

// ReaderFromReaderAt takes an io.ReaderAt and returns an io.Reader
func ReaderFromReaderAt(r io.ReaderAt) io.Reader {
	return &readerAtReader{
		r:   r,
		off: 0,
	}
}

func (rar *readerAtReader) Read(p []byte) (n int, err error) {
	n, err = rar.r.ReadAt(p, rar.off)
	rar.off += int64(n)
	return n, err
}
