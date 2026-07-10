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

package fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/containerd/continuity/devices"
	"github.com/containerd/continuity/sysx"

	"golang.org/x/sys/unix"
)

const (
	// whiteoutPrefix prefix means file is a whiteout. If this is followed
	// by a filename this means that file has been removed from the base
	// layer.
	//
	// See https://github.com/opencontainers/image-spec/blob/master/layer.md#whiteouts
	whiteoutPrefix = ".wh."
)

// overlayFSWhiteoutConvert detects whiteouts and opaque directories.
//
// It returns deleted indicator if the file is a character device with 0/0
// device number. And call changeFn with ChangeKindDelete for opaque
// directories.
//
// Check: https://www.kernel.org/doc/Documentation/filesystems/overlayfs.txt
func overlayFSWhiteoutConvert(diffDir, path string, f os.FileInfo, changeFn ChangeFunc) (deleted bool, _ error) {
	if f.Mode()&os.ModeCharDevice != 0 {
		if _, ok := f.Sys().(*syscall.Stat_t); !ok {
			return false, nil
		}

		maj, min, err := devices.DeviceInfo(f)
		if err != nil {
			return false, err
		}
		return (maj == 0 && min == 0), nil
	}

	if f.IsDir() {
		originalPath := filepath.Join(diffDir, path)
		opaque, err := getOpaqueValue(originalPath)
		if err != nil {
			if errors.Is(err, unix.ENODATA) {
				return false, nil
			}
			return false, err
		}

		if len(opaque) == 1 && opaque[0] == 'y' {
			opaqueDirPath := filepath.Join(path, whiteoutPrefix+".opq")
			return false, changeFn(ChangeKindDelete, opaqueDirPath, nil, nil)
		}
	}
	return false, nil
}

// getOpaqueValue returns opaque value for a given file.
func getOpaqueValue(filePath string) ([]byte, error) {
	for _, xattr := range []string{
		"trusted.overlay.opaque",
		// TODO(fuweid):
		//
		// user.overlay.* is available since 5.11. We should check
		// kernel version before read.
		//
		// REF: https://github.com/torvalds/linux/commit/2d2f2d7322ff43e0fe92bf8cccdc0b09449bf2e1
		"user.overlay.opaque",
	} {
		opaque, err := sysx.LGetxattr(filePath, xattr)
		if err != nil {
			if errors.Is(err, unix.ENODATA) || errors.Is(err, unix.ENOTSUP) {
				continue
			}
			return nil, fmt.Errorf("failed to retrieve %s attr: %w", xattr, err)
		}
		return opaque, nil
	}
	return nil, unix.ENODATA
}
