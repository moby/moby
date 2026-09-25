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

package fsview

import "io/fs"

func getxattr(f fs.File, name string) (string, bool) {
	for _, h := range registered {
		if h.Getxattr != nil {
			if val, ok := h.Getxattr(f, name); ok {
				return val, true
			}
		}
	}
	return "", false
}

func isOpaque(f fs.File) bool {
	for _, xattr := range OverlayOpaqueXattrs {
		if val, ok := getxattr(f, xattr); ok && val == "y" {
			return true
		}
	}
	return false
}

func isWhiteout(fi fs.FileInfo) bool {
	if (fi.Mode() & fs.ModeCharDevice) == 0 {
		return false
	}
	for _, h := range registered {
		if h.IsWhiteout != nil && h.IsWhiteout(fi) {
			return true
		}
	}
	return false
}
