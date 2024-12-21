//go:build !windows

package system // import "github.com/docker/docker/pkg/system"

import "os"

// MkdirAllWithACL is a wrapper for os.MkdirAll on unix systems.
func MkdirAllWithACL(path string, perm os.FileMode, _ string) error {
	return os.MkdirAll(path, perm)
}
