// +build linux

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

import "golang.org/x/sys/unix"

// EpollCreate1 directly calls unix.EpollCreate1
func EpollCreate1(flag int) (int, error) {
	return unix.EpollCreate1(flag)
}

// EpollCtl directly calls unix.EpollCtl
func EpollCtl(epfd int, op int, fd int, event *unix.EpollEvent) error {
	return unix.EpollCtl(epfd, op, fd, event)
}

// EpollWait directly calls unix.EpollWait
func EpollWait(epfd int, events []unix.EpollEvent, msec int) (int, error) {
	return unix.EpollWait(epfd, events, msec)
}
