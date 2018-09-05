// +build !windows

package system // import "github.com/docker/docker/pkg/system"

import (
	"os"
	"syscall"
)

// Lstat takes a path to a file and returns
// a system.StatT type pertaining to that file.
//
// Throws an error if the file does not exist
func Lstat(path string) (*StatT, error) {
	s := &syscall.Stat_t{}
	if err := syscall.Lstat(path, s); err != nil {
		return nil, err
	}
	return fromStatT(s)
}

// GetFileInfo on non-Windows simply invokes Lstat.
//
// Throws an error if the file does not exist
func GetFileInfo(path string) (os.FileInfo, error) {
	fi, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	return fi, nil
}
