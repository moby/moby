// +build !windows

package system

import (
	"os"
	"path/filepath"
)

// MkdirAllWithACL is a wrapper for MkdirAll that creates a directory
// ACL'd for Builtin Administrators and Local System.
func MkdirAllWithACL(path string, perm os.FileMode) error {
	return MkdirAll(path, perm)
}

// MkdirAll creates a directory named path along with any necessary parents,
// with permission specified by attribute perm for all dir created.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// IsAbs is a platform-specific wrapper for filepath.IsAbs.
func IsAbs(path string) bool {
	return filepath.IsAbs(path)
}
