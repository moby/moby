// +build !windows

package system

import (
	"os"
)

// MkdirAll creates a directory named path along with any necessary parents,
// with permission specified by attribute perm for all dir created.
func MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
