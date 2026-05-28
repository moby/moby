package fs

import (
	"os"
	"syscall"
	"time"
)

// MkdirAll is forked os.MkdirAll
func MkdirAll(path string, perm os.FileMode, user Chowner, tm *time.Time) ([]string, error) {
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := os.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil, nil
		}
		return nil, &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
		j--
	}

	var createdDirs []string

	if j > 1 {
		// Create parent.
		createdDirs, err = MkdirAll(fixRootDirectory(path[:j-1]), perm, user, tm)
		if err != nil {
			return nil, err
		}
	}

	dir, err1 := os.Lstat(path)
	if err1 == nil && dir.IsDir() {
		return createdDirs, nil
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = os.Mkdir(path, perm)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := os.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return createdDirs, nil
		}
		return nil, err
	}
	createdDirs = append(createdDirs, path)

	if err := Chown(path, nil, user); err != nil {
		return nil, err
	}

	if err := Utimes(path, tm); err != nil {
		return nil, err
	}

	return createdDirs, nil
}
