//go:build !windows

package system

import "os"

// MkdirAllWithACL is a wrapper for os.MkdirAll on unix systems.
func MkdirAllWithACL(path string, perm os.FileMode, _ string) error {
	return os.MkdirAll(path, perm)
}
