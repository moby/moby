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

package mount

import (
	"os/exec"

	"golang.org/x/sys/unix"
)

// fuseSuperMagic is defined in statfs(2)
const fuseSuperMagic = 0x65735546

func isFUSE(dir string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(dir, &st); err != nil {
		return false
	}
	return st.Type == fuseSuperMagic
}

// unmountFUSE attempts to unmount using fusermount/fusermount3 helper binary.
//
// For FUSE mounts, using these helper binaries is preferred, see:
// https://github.com/containerd/containerd/pull/3765#discussion_r342083514
func unmountFUSE(target string) error {
	var err error
	for _, helperBinary := range []string{"fusermount3", "fusermount"} {
		cmd := exec.Command(helperBinary, "-u", target)
		err = cmd.Run()
		if err == nil {
			return nil
		}
	}
	return err
}
