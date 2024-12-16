//go:build !windows

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"syscall"
)

// Lstat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist.
//
// Deprecated: this function is only used internally, and will be removed in the next release.
func Lstat(path string) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Lstat(path, s); err != nil {
		return nil, &os.PathError{Op: "Lstat", Path: path, Err: err}
	}
	return fromStatT(s)
}
