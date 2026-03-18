// This code mostly comes from <https://github.com/cyphar/filepath-securejoin>.

// Copyright (C) 2014-2015 Docker Inc & Go Authors. All rights reserved.
// Copyright (C) 2017-2024 SUSE LLC. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package contenthash

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

var errTooManyLinks = errors.New("too many links")

const maxSymlinkLimit = 255

type onSymlinkFunc func(string, string) error

// rootPath joins a path with a root, evaluating and bounding any symlink to
// the root directory. This is a slightly modified version of SecureJoin from
// github.com/cyphar/filepath-securejoin, with a callback which we call after
// each symlink resolution.
func rootPath(root, unsafePath string, followTrailing bool, cb onSymlinkFunc) (string, error) {
	if unsafePath == "" {
		return root, nil
	}

	unsafePath = filepath.FromSlash(unsafePath)
	var (
		currentPath string
		linksWalked int
	)
	for unsafePath != "" {
		// Windows-specific: remove any drive letters from the path.
		if v := filepath.VolumeName(unsafePath); v != "" {
			unsafePath = unsafePath[len(v):]
		}

		// Remove any unnecessary trailing slashes.
		unsafePath = strings.TrimSuffix(unsafePath, string(filepath.Separator))

		// Get the next path component.
		var part string
		if i := strings.IndexRune(unsafePath, filepath.Separator); i == -1 {
			part, unsafePath = unsafePath, ""
		} else {
			part, unsafePath = unsafePath[:i], unsafePath[i+1:]
		}

		// Apply the component lexically to the path we are building. path does
		// not contain any symlinks, and we are lexically dealing with a single
		// component, so it's okay to do filepath.Clean here.
		nextPath := filepath.Join(string(filepath.Separator), currentPath, part)
		if nextPath == string(filepath.Separator) {
			// If we end up back at the root, we don't need to re-evaluate /.
			currentPath = ""
			continue
		}
		fullPath := root + string(filepath.Separator) + nextPath

		// Figure out whether the path is a symlink.
		fi, err := os.Lstat(fullPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		// Treat non-existent path components the same as non-symlinks (we
		// can't do any better here).
		if errors.Is(err, os.ErrNotExist) || fi.Mode()&os.ModeSymlink == 0 {
			currentPath = nextPath
			continue
		}
		// Don't resolve the final component with !followTrailing.
		if !followTrailing && unsafePath == "" {
			currentPath = nextPath
			break
		}

		// It's a symlink, so get its contents and expand it by prepending it
		// to the yet-unparsed path.
		linksWalked++
		if linksWalked > maxSymlinkLimit {
			return "", errTooManyLinks
		}

		dest, err := os.Readlink(fullPath)
		if err != nil {
			return "", err
		}
		if cb != nil {
			if err := cb(nextPath, dest); err != nil {
				return "", err
			}
		}

		unsafePath = dest + string(filepath.Separator) + unsafePath
		// Absolute symlinks reset any work we've already done.
		if filepath.IsAbs(dest) {
			currentPath = ""
		}
	}

	// There should be no lexical components left in path here, but just for
	// safety do a filepath.Clean before the join.
	finalPath := filepath.Join(string(filepath.Separator), currentPath)
	return filepath.Join(root, finalPath), nil
}
