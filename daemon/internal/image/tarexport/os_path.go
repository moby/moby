// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Code in this file is a modified version of go stdlib;
// https://cs.opensource.google/go/go/+/refs/tags/go1.23.4:src/os/path.go;l=19-66

package tarexport

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/moby/moby/v2/daemon/internal/system"
)

// mkdirAllWithChtimes is nearly an identical copy to the [os.MkdirAll] but
// tracks created directories and applies the provided mtime and atime using
// [system.Chtimes].
func mkdirAllWithChtimes(path string, perm os.FileMode, atime, mtime time.Time) error {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.

	// Extract the parent folder from path by first removing any trailing
	// path separator and then scanning backward until finding a path
	// separator or reaching the beginning of the string.
	i := len(path) - 1
	for i >= 0 && os.IsPathSeparator(path[i]) {
		i--
	}
	for i >= 0 && !os.IsPathSeparator(path[i]) {
		i--
	}
	if i < 0 {
		i = 0
	}

	// If there is a parent directory, and it is not the volume name,
	// recurse to ensure parent directory exists.
	if parent := path[:i]; len(parent) > len(filepath.VolumeName(path)) {
		err = mkdirAllWithChtimes(parent, perm, atime, mtime)
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = os.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}

	if err := system.Chtimes(path, atime, mtime); err != nil {
		return fmt.Errorf("applying atime=%v and mtime=%v: %w", atime, mtime, err)
	}
	return nil
}
