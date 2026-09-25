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

package fsview

import (
	"io/fs"

	"github.com/containerd/containerd/v2/core/mount"
)

// FSHandler extends fsview with support for additional filesystem types.
// All fields are optional — set only the capabilities the handler provides.
type FSHandler struct {
	// HandleMount converts a mount into a View. It should return
	// errdefs.ErrNotImplemented if it cannot handle the mount type.
	HandleMount func(m mount.Mount) (View, error)

	// Getxattr returns the value of the named extended attribute on the
	// given file. The boolean indicates whether the attribute was found.
	Getxattr func(f fs.File, name string) (string, bool)

	// IsWhiteout checks if a file info represents a whiteout entry.
	IsWhiteout func(fi fs.FileInfo) bool
}

var registered []FSHandler

// Register adds a filesystem handler to extend fsview with support
// for additional filesystem types.
func Register(h FSHandler) {
	registered = append(registered, h)
}
