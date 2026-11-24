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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

var errTooManyLinks = errors.New("too many links")

type fsRootPathResult struct {
	path                         string
	followedAbsoluteLink         bool
	relativeEscapeBeforeAbsolute bool
}

// fsRootPath joins a path with a root, evaluating and bounding any
// symlink to the root directory.
func fsRootPath(root, path string) (string, error) {
	result, err := resolveFSRootPath(root, path)
	if err != nil {
		return "", err
	}
	return result.path, nil
}

func resolveFSRootPath(root, path string) (fsRootPathResult, error) {
	result := fsRootPathResult{path: root}
	parts := strings.Split(filepath.FromSlash(path), string(os.PathSeparator))
	resolved := make([]string, 0, len(parts))
	linksWalked := 0
	for len(parts) > 0 {
		part := parts[0]
		parts = parts[1:]
		switch part {
		case "", ".":
			continue
		case "..":
			if len(resolved) > 0 {
				resolved = resolved[:len(resolved)-1]
			} else if !result.followedAbsoluteLink {
				// Keep the path bounded, but do not let a later absolute link
				// hide a relative escape from resolveArchivePath.
				result.relativeEscapeBeforeAbsolute = true
			}
			continue
		}

		candidate := filepath.Join(root, filepath.Join(append(resolved, part)...))
		fi, err := os.Lstat(candidate)
		if os.IsNotExist(err) {
			// A later .. can return to an existing parent. Continue resolving
			// instead of cleaning the remaining path and skipping its symlinks.
			resolved = append(resolved, part)
			continue
		}
		if err != nil {
			return fsRootPathResult{}, err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			if len(parts) > 0 && !fi.IsDir() {
				return fsRootPathResult{}, &os.PathError{Op: "resolve", Path: candidate, Err: syscall.ENOTDIR}
			}
			resolved = append(resolved, part)
			continue
		}
		if linksWalked == 255 {
			return fsRootPathResult{}, errTooManyLinks
		}
		linksWalked++
		target, err := os.Readlink(candidate)
		if err != nil {
			return fsRootPathResult{}, err
		}
		if filepath.IsAbs(target) {
			result.followedAbsoluteLink = true
			resolved = resolved[:0]
		}
		// Expand symlinks before processing a following ..; lexical cleaning
		// would remove the link rather than the directory it resolves to.
		parts = append(strings.Split(filepath.FromSlash(target), string(os.PathSeparator)), parts...)
	}
	result.path = filepath.Join(root, filepath.Join(resolved...))
	return result, nil
}
