package fileutils

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd/log"
)

// GetTotalUsedFds Returns the number of used File Descriptors by
// reading it via /proc filesystem.
func GetTotalUsedFds() int {
	name := fmt.Sprintf("/proc/%d/fd", os.Getpid())
	f, err := os.Open(name)
	if err != nil {
		log.G(context.TODO()).WithError(err).Error("Error listing file descriptors")
		return -1
	}
	defer f.Close()

	var fdCount int
	for {
		names, err := f.Readdirnames(100)
		fdCount += len(names)
		if err == io.EOF {
			break
		} else if err != nil {
			log.G(context.TODO()).WithError(err).Error("Error listing file descriptors")
			return -1
		}
	}
	return fdCount
}
