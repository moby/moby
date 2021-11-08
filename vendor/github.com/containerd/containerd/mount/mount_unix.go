//go:build darwin || openbsd
// +build darwin openbsd

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

import "github.com/pkg/errors"

var (
	// ErrNotImplementOnUnix is returned for methods that are not implemented
	ErrNotImplementOnUnix = errors.New("not implemented under unix")
)

// Mount is not implemented on this platform
func (m *Mount) Mount(target string) error {
	return ErrNotImplementOnUnix
}

// Unmount is not implemented on this platform
func Unmount(mount string, flags int) error {
	return ErrNotImplementOnUnix
}

// UnmountAll is not implemented on this platform
func UnmountAll(mount string, flags int) error {
	return ErrNotImplementOnUnix
}
