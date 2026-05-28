//go:build !windows

package system

import "path/filepath"

// IsAbsolutePath is just a wrapper that calls filepath.IsAbs.
// Has been added here just for symmetry with Windows.
func IsAbsolutePath(path string) bool {
	return filepath.IsAbs(path)
}

// GetAbsolutePath does nothing on non-Windows, just returns
// the same path.
func GetAbsolutePath(path string) string {
	return path
}
