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

// EpollCreate1 is an alias for unix.EpollCreate1
// Deprecated: use golang.org/x/sys/unix.EpollCreate1
var EpollCreate1 = unix.EpollCreate1

// EpollCtl is an alias for unix.EpollCtl
// Deprecated: use golang.org/x/sys/unix.EpollCtl
var EpollCtl = unix.EpollCtl

// EpollWait is an alias for unix.EpollWait
// Deprecated: use golang.org/x/sys/unix.EpollWait
var EpollWait = unix.EpollWait
