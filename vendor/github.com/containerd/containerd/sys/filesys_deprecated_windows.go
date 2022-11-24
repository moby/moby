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

package sys

import (
	"os"

	"github.com/moby/sys/sequential"
)

// CreateSequential is deprecated.
//
// Deprecated: use github.com/moby/sys/sequential.Create
func CreateSequential(name string) (*os.File, error) {
	return sequential.Create(name)
}

// OpenSequential is deprecated.
//
// Deprecated: use github.com/moby/sys/sequential.Open
func OpenSequential(name string) (*os.File, error) {
	return sequential.Open(name)
}

// OpenFileSequential is deprecated.
//
// Deprecated: use github.com/moby/sys/sequential.OpenFile
func OpenFileSequential(name string, flag int, perm os.FileMode) (*os.File, error) {
	return sequential.OpenFile(name, flag, perm)
}

// TempFileSequential is deprecated.
//
// Deprecated: use github.com/moby/sys/sequential.CreateTemp
func TempFileSequential(dir, prefix string) (f *os.File, err error) {
	return sequential.CreateTemp(dir, prefix)
}
