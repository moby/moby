// +build freebsd

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

package archive

import (
	"os"

	"golang.org/x/sys/unix"
)

// mknod wraps unix.Mknod.  FreeBSD's unix.Mknod signature is different from
// other Unix and Unix-like operating systems.
func mknod(path string, mode uint32, dev uint64) error {
	return unix.Mknod(path, mode, dev)
}

// lsetxattrCreate wraps unix.Lsetxattr with FreeBSD-specific flags and errors
func lsetxattrCreate(link string, attr string, data []byte) error {
	err := unix.Lsetxattr(link, attr, data, 0)
	if err == unix.ENOTSUP || err == unix.EEXIST {
		return nil
	}
	return err
}

func lchmod(path string, mode os.FileMode) error {
	err := unix.Fchmodat(unix.AT_FDCWD, path, uint32(mode), unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		err = &os.PathError{Op: "lchmod", Path: path, Err: err}
	}
	return err
}
