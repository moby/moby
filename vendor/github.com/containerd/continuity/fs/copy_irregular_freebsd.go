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
	"os"
	"syscall"
)

// copyIrregular covers devices, pipes, and sockets
func copyIrregular(dst string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t) // not *unix.Stat_t
	if !ok {
		return fmt.Errorf("unsupported stat type: %s: %v", dst, fi.Mode())
	}
	var rDev uint64 // uint64 on FreeBSD, int on other unixen
	if fi.Mode()&os.ModeDevice == os.ModeDevice {
		rDev = st.Rdev
	}
	return syscall.Mknod(dst, uint32(st.Mode), rDev)
}
