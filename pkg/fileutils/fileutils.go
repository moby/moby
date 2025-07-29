package fileutils

import (
	"fmt"
	"os"
	"path/filepath"
)

// ReadSymlinkedDirectory returns the target directory of a symlink.
// The target of the symbolic link may not be a file.
func ReadSymlinkedDirectory(path string) (realPath string, _ error) {
	var err error
	realPath, err = filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("unable to get absolute path for %s: %w", path, err)
	}
	realPath, err = filepath.EvalSymlinks(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to canonicalise path for %s: %w", path, err)
	}
	realPathInfo, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("failed to stat target '%s' of '%s': %w", realPath, path, err)
	}
	if !realPathInfo.Mode().IsDir() {
		return "", fmt.Errorf("canonical path points to a file '%s'", realPath)
	}
	return realPath, nil
}
