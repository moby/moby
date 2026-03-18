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
	"fmt"
	"io/fs"
	"syscall"
	"time"
)

func Atime(st fs.FileInfo) (time.Time, error) {
	stSys, ok := st.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return time.Time{}, fmt.Errorf("expected st.Sys() to be *syscall.Win32FileAttributeData, got %T", st.Sys())
	}
	// ref: https://github.com/golang/go/blob/go1.19.2/src/os/types_windows.go#L230
	return time.Unix(0, stSys.LastAccessTime.Nanoseconds()), nil
}
