/*
   Copyright The containerd Authors.

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

package fs

import (
	"io"
	"os"
)

type dirReader struct {
	buf []os.DirEntry
	f   *os.File
	err error
}

func (r *dirReader) Next() os.DirEntry {
	if len(r.buf) == 0 {
		infos, err := r.f.ReadDir(32)
		if err != nil {
			if err != io.EOF {
				r.err = err
			}
			return nil
		}
		r.buf = infos
	}

	if len(r.buf) == 0 {
		return nil
	}
	out := r.buf[0]
	r.buf[0] = nil
	r.buf = r.buf[1:]
	return out
}

func (r *dirReader) Err() error {
	return r.err
}
