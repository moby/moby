//go:build !linux

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

package erofs

import (
	"context"

	"github.com/containerd/errdefs"
)

// defaultWritableSize is the default size allocation for writable
// layers on non-Linux platforms. This is set to 64MiB but may be
// adjusted in the user configuration or per snapshot.
const defaultWritableSize = 64 * 1024 * 1024

func checkCompatibility(root string) error {
	return nil
}

func setImmutable(path string, enable bool) error {
	return errdefs.ErrNotImplemented
}

func cleanupUpper(upper string) error {
	return nil
}

func convertDirToErofs(ctx context.Context, layerBlob, upperDir string) error {
	return errdefs.ErrNotImplemented
}

func getParentOwnership(parentPath string) (uid, gid int, err error) {
	return -1, -1, nil
}
