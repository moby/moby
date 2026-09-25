//go:build !linux && !windows

package archive

import (
	"os"

	"github.com/moby/go-archive/internal/archiveoptions"
	"golang.org/x/sys/unix"
)

// chmodNoSymlinkFallback applies mode without following the final path
// component on systems without fchmodat2 support.
//
// Callers must have already excluded symlink entries.
func chmodNoSymlinkFallback(parentFD int, base, name string, perm uint32, _ *archiveoptions.Options) error {
	fd, err := unix.Openat(parentFD, base, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "openat", Path: name, Err: err}
	}
	defer unix.Close(fd)

	if err := unix.Fchmod(fd, perm); err != nil {
		return &os.PathError{Op: "fchmod", Path: name, Err: err}
	}
	return nil
}
