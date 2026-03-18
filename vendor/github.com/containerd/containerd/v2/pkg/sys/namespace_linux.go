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
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

// GetUsernsForNamespace returns a file descriptor that refers to the owning
// user namespace for the namespace referred to by fd.
//
// REF: https://man7.org/linux/man-pages/man2/ioctl_ns.2.html
func GetUsernsForNamespace(fd uintptr) (*os.File, error) {
	fd, _, errno := unix.Syscall(syscall.SYS_IOCTL, fd, uintptr(unix.NS_GET_USERNS), 0)
	if errno != 0 {
		return nil, fmt.Errorf("failed to get user namespace fd: %w", errno)
	}

	return os.NewFile(fd, fmt.Sprintf("/proc/%d/fd/%d", os.Getpid(), fd)), nil
}
