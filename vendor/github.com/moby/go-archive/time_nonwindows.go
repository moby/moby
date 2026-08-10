//go:build !windows

package archive

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// chtimes changes the access and modification time of a file at the given
// path relative to root.
//
// Callers must use boundTime to ensure timestamps are within the range
// supported by os.Chtimes.
func chtimes(root *os.Root, name string, atime, mtime time.Time) error {
	return root.Chtimes(name, atime, mtime)
}

func lchtimes(root *os.Root, name string, atime, mtime time.Time) error {
	dir, base := path.Split(filepath.ToSlash(name))
	if base == "" {
		return &os.PathError{Op: "lchtimes", Path: name, Err: syscall.EINVAL}
	}

	dir = strings.TrimSuffix(dir, "/")
	if dir == "" {
		dir = "."
	}

	parent, err := root.Open(dir)
	if err != nil {
		return err
	}
	defer parent.Close()

	utimes := [2]unix.Timespec{
		timeToTimespec(atime),
		timeToTimespec(mtime),
	}
	// #nosec G115 -- ignore integer overflow conversion for parent.Fd
	if err := unix.UtimesNanoAt(int(parent.Fd()), base, utimes[:], unix.AT_SYMLINK_NOFOLLOW); err != nil {
		if errors.Is(err, unix.ENOSYS) {
			return nil
		}
		return &os.PathError{Op: "lchtimes", Path: name, Err: err}
	}
	return nil
}

func timeToTimespec(time time.Time) unix.Timespec {
	if time.IsZero() {
		// Return UTIME_OMIT special value
		return unix.Timespec{
			Sec:  0,
			Nsec: (1 << 30) - 2,
		}
	}
	return unix.NsecToTimespec(time.UnixNano())
}
