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
	"unsafe"

	"golang.org/x/sys/unix"
)

// If arg2 is nonzero, set the "child subreaper" attribute of the
// calling process; if arg2 is zero, unset the attribute.  When a
// process is marked as a child subreaper, all of the children
// that it creates, and their descendants, will be marked as
// having a subreaper.  In effect, a subreaper fulfills the role
// of init(1) for its descendant processes.  Upon termination of
// a process that is orphaned (i.e., its immediate parent has
// already terminated) and marked as having a subreaper, the
// nearest still living ancestor subreaper will receive a SIGCHLD
// signal and be able to wait(2) on the process to discover its
// termination status.
const setChildSubreaper = 36

// SetSubreaper sets the value i as the subreaper setting for the calling process
func SetSubreaper(i int) error {
	return unix.Prctl(setChildSubreaper, uintptr(i), 0, 0, 0)
}

// GetSubreaper returns the subreaper setting for the calling process
func GetSubreaper() (int, error) {
	var i uintptr

	if err := unix.Prctl(unix.PR_GET_CHILD_SUBREAPER, uintptr(unsafe.Pointer(&i)), 0, 0, 0); err != nil {
		return -1, err
	}

	return int(i), nil
}
