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

// chtimes changes the access time and modified time of a file at the given path.
// If the modified time is prior to the Unix Epoch (unixMinTime), or after the
// end of Unix Time (unixEpochTime), os.Chtimes has undefined behavior. In this
// case, Chtimes defaults to Unix Epoch, just in case.
func chtimes(name string, atime time.Time, mtime time.Time) error {
	return os.Chtimes(name, atime, mtime)
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
