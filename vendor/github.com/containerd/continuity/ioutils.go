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

package continuity

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

// AtomicWriteFile atomically writes data to a file by first writing to a
// temp file and calling rename.
func AtomicWriteFile(filename string, data []byte, perm os.FileMode) error {
	buf := bytes.NewBuffer(data)
	return atomicWriteFile(filename, buf, int64(len(data)), perm)
}

// atomicWriteFile writes data to a file by first writing to a temp
// file and calling rename.
func atomicWriteFile(filename string, r io.Reader, dataSize int64, perm os.FileMode) error {
	f, err := ioutil.TempFile(filepath.Dir(filename), ".tmp-"+filepath.Base(filename))
	if err != nil {
		return err
	}
	err = os.Chmod(f.Name(), perm)
	if err != nil {
		f.Close()
		return err
	}
	n, err := io.Copy(f, r)
	if err == nil && n < dataSize {
		f.Close()
		return io.ErrShortWrite
	}
	if err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filename)
}
