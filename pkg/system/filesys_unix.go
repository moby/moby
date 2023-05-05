//go:build !windows

package system // import "github.com/docker/docker/pkg/system"

import "os"

// MkdirAllWithACL is a wrapper for os.MkdirAll on unix systems.
func MkdirAllWithACL(path string, perm os.FileMode, sddl string) error {
	return os.MkdirAll(path, perm)
}

// MkdirAll creates a directory named path along with any necessary parents,
// with permission specified by attribute perm for all dir created.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
