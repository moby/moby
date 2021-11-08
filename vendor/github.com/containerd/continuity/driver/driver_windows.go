// +build go1.13

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

// Go 1.13 is the minimally supported version for Windows.
// Earlier golang releases have bug in os.Readlink
// (see https://github.com/golang/go/issues/30463).

package driver

import (
	"os"
)

func (d *driver) Mknod(path string, mode os.FileMode, major, minor int) error {
	return &os.PathError{Op: "mknod", Path: path, Err: ErrNotSupported}
}

func (d *driver) Mkfifo(path string, mode os.FileMode) error {
	return &os.PathError{Op: "mkfifo", Path: path, Err: ErrNotSupported}
}

// Lchmod changes the mode of an file not following symlinks.
func (d *driver) Lchmod(path string, mode os.FileMode) (err error) {
	// TODO: Use Window's equivalent
	return os.Chmod(path, mode)
}
