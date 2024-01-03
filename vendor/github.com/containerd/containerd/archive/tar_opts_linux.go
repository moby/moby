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
	"archive/tar"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// AufsConvertWhiteout converts whiteout files for aufs.
func AufsConvertWhiteout(_ *tar.Header, _ string) (bool, error) {
	return true, nil
}

// OverlayConvertWhiteout converts whiteout files for overlay.
func OverlayConvertWhiteout(hdr *tar.Header, path string) (bool, error) {
	base := filepath.Base(path)
	dir := filepath.Dir(path)

	// if a directory is marked as opaque, we need to translate that to overlay
	if base == whiteoutOpaqueDir {
		// don't write the file itself
		return false, unix.Setxattr(dir, "trusted.overlay.opaque", []byte{'y'}, 0)
	}

	// if a file was deleted and we are using overlay, we need to create a character device
	if strings.HasPrefix(base, whiteoutPrefix) {
		originalBase := base[len(whiteoutPrefix):]
		originalPath := filepath.Join(dir, originalBase)

		if err := unix.Mknod(originalPath, unix.S_IFCHR, 0); err != nil {
			return false, err
		}
		// don't write the file itself
		return false, os.Chown(originalPath, hdr.Uid, hdr.Gid)
	}

	return true, nil
}
