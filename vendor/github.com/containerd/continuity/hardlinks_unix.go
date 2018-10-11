// +build linux darwin freebsd solaris

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
	"fmt"
	"os"
	"syscall"
)

// hardlinkKey provides a tuple-key for managing hardlinks. This is system-
// specific.
type hardlinkKey struct {
	dev   uint64
	inode uint64
}

// newHardlinkKey returns a hardlink key for the provided file info. If the
// resource does not represent a possible hardlink, errNotAHardLink will be
// returned.
func newHardlinkKey(fi os.FileInfo) (hardlinkKey, error) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return hardlinkKey{}, fmt.Errorf("cannot resolve (*syscall.Stat_t) from os.FileInfo")
	}

	if sys.Nlink < 2 {
		// NOTE(stevvooe): This is not always true for all filesystems. We
		// should somehow detect this and provided a slow "polyfill" that
		// leverages os.SameFile if we detect a filesystem where link counts
		// is not really supported.
		return hardlinkKey{}, errNotAHardLink
	}

	return hardlinkKey{dev: uint64(sys.Dev), inode: uint64(sys.Ino)}, nil
}
