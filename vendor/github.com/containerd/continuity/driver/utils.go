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

package driver

import (
	"io"
	"io/ioutil"
	"os"
	"sort"
)

// ReadFile works the same as ioutil.ReadFile with the Driver abstraction
func ReadFile(r Driver, filename string) ([]byte, error) {
	f, err := r.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// WriteFile works the same as ioutil.WriteFile with the Driver abstraction
func WriteFile(r Driver, filename string, data []byte, perm os.FileMode) error {
	f, err := r.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer f.Close()

	n, err := f.Write(data)
	if err != nil {
		return err
	} else if n != len(data) {
		return io.ErrShortWrite
	}

	return nil
}

// ReadDir works the same as ioutil.ReadDir with the Driver abstraction
func ReadDir(r Driver, dirname string) ([]os.FileInfo, error) {
	f, err := r.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dirs, err := f.Readdir(-1)
	if err != nil {
		return nil, err
	}

	sort.Sort(fileInfos(dirs))
	return dirs, nil
}

// Simple implementation of the sort.Interface for os.FileInfo
type fileInfos []os.FileInfo

func (fis fileInfos) Len() int {
	return len(fis)
}

func (fis fileInfos) Less(i, j int) bool {
	return fis[i].Name() < fis[j].Name()
}

func (fis fileInfos) Swap(i, j int) {
	fis[i], fis[j] = fis[j], fis[i]
}
