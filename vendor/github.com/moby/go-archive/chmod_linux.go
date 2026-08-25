package archive

import (
	"fmt"
	"os"
	"runtime"
	"strconv"

	"github.com/moby/go-archive/internal/archiveoptions"
	"golang.org/x/sys/unix"
)

// chmodNoSymlinkFallback applies mode without following the final path
// component on systems without fchmodat2 support.
//
// Callers must have already excluded symlink entries.
func chmodNoSymlinkFallback(parentFD int, base, name string, perm uint32, opts *archiveoptions.Options) error {
	fd, err := unix.Openat(parentFD, base, unix.O_PATH|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "openat", Path: name, Err: err}
	}
	defer unix.Close(fd)

	if opts != nil && opts.ProcSelfFD != nil {
		err := unix.Fchmodat(int(opts.ProcSelfFD.Fd()), strconv.Itoa(fd), perm, 0)
		// Keep the os.File alive until fchmodat has finished using its descriptor.
		runtime.KeepAlive(opts.ProcSelfFD)
		if err != nil {
			return &os.PathError{
				Op:   "fchmodat",
				Path: name,
				Err:  fmt.Errorf("via pre-opened /proc/self/fd/%d: %w", fd, err),
			}
		}
	} else {
		procPath := "/proc/self/fd/" + strconv.Itoa(fd)
		if err := unix.Chmod(procPath, perm); err != nil {
			return &os.PathError{
				Op:   "chmod",
				Path: name,
				Err:  fmt.Errorf("via %s: %w", procPath, err),
			}
		}
	}
	return nil
}
