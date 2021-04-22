// +build !windows,!freebsd

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

import "golang.org/x/sys/unix"

// mknod wraps Unix.Mknod and casts dev to int
func mknod(path string, mode uint32, dev uint64) error {
	return unix.Mknod(path, mode, int(dev))
}

// lsetxattrCreate wraps unix.Lsetxattr, passes the unix.XATTR_CREATE flag on
// supported operating systems,and ignores appropriate errors
func lsetxattrCreate(link string, attr string, data []byte) error {
	err := unix.Lsetxattr(link, attr, data, unix.XATTR_CREATE)
	if err == unix.ENOTSUP || err == unix.ENODATA || err == unix.EEXIST {
		return nil
	}
	return err
}
