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

package local

import (
	"os"
)

// readerat implements io.ReaderAt in a completely stateless manner by opening
// the referenced file for each call to ReadAt.
type sizeReaderAt struct {
	size int64
	fp   *os.File
}

func (ra sizeReaderAt) ReadAt(p []byte, offset int64) (int, error) {
	return ra.fp.ReadAt(p, offset)
}

func (ra sizeReaderAt) Size() int64 {
	return ra.size
}

func (ra sizeReaderAt) Close() error {
	return ra.fp.Close()
}
