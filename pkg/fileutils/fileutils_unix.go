//go:build linux || freebsd
// +build linux freebsd

package fileutils // import "github.com/docker/docker/pkg/fileutils"

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"
)

// GetTotalUsedFds Returns the number of used File Descriptors by
// reading it via /proc filesystem.
func GetTotalUsedFds() int {
	if fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", os.Getpid())); err != nil {
		logrus.Errorf("Error opening /proc/%d/fd: %s", os.Getpid(), err)
	} else {
		return len(fds)
	}
	return -1
}
