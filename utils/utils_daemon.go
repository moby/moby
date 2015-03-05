// +build daemon

package utils

import (
	"os"
	"syscall"
)

// IsFileOwner checks whether the current user is the owner of the given file.
func IsFileOwner(f string) bool {
	if fileInfo, err := os.Stat(f); err == nil && fileInfo != nil {
		if stat, ok := fileInfo.Sys().(*syscall.Stat_t); ok && int(stat.Uid) == os.Getuid() {
			return true
		}
	}
	return false
}
