// +build daemon

package utils

import (
	"github.com/docker/docker/pkg/system"
	"os"
)

// IsFileOwner checks whether the current user is the owner of the given file.
func IsFileOwner(f string) bool {
	if fileInfo, err := system.Stat(f); err == nil && fileInfo != nil {
		if int(fileInfo.Uid()) == os.Getuid() {
			return true
		}
	}
	return false
}
