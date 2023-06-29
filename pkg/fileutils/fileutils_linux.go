package fileutils

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/containerd/containerd/log"
	"golang.org/x/sys/unix"
)

// GetTotalUsedFds Returns the number of used File Descriptors by
// reading it via /proc filesystem.
func GetTotalUsedFds() int {
	name := fmt.Sprintf("/proc/%d/fd", os.Getpid())

	// Fast-path for Linux 6.2 (since [f1f1f2569901ec5b9d425f2e91c09a0e320768f3]).
	// From the [Linux docs]:
	//
	// "The number of open files for the process is stored in 'size' member of
	// stat() output for /proc/<pid>/fd for fast access."
	//
	// [Linux docs]: https://docs.kernel.org/filesystems/proc.html#proc-pid-fd-list-of-symlinks-to-open-files:
	// [f1f1f2569901ec5b9d425f2e91c09a0e320768f3]: https://github.com/torvalds/linux/commit/f1f1f2569901ec5b9d425f2e91c09a0e320768f3
	var stat unix.Stat_t
	if err := unix.Stat(name, &stat); err == nil && stat.Size > 0 {
		return int(stat.Size)
	}

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
	// Note that the slow path has 1 more file-descriptor, due to the open
	// file-handle for /proc/<pid>/fd during the calculation.
	return fdCount
}
