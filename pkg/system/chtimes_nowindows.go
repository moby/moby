//go:build !windows
// +build !windows

package system // import "github.com/docker/docker/pkg/system"

import (
	"time"

	"golang.org/x/sys/unix"
)

// setCTime will set the create time on a file. On Unix, the create
// time is updated as a side effect of setting the modified time, so
// no action is required.
func setCTime(path string, ctime time.Time) error {
	return nil
}

// setAMTimeNoFollow will set access/modification time on a file,
// without following symbol link.
func setAMTimeNoFollow(path string, atime time.Time, mtime time.Time) error {
	uts := []unix.Timespec{
		unix.NsecToTimespec(atime.UnixNano()),
		unix.NsecToTimespec(mtime.UnixNano()),
	}
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, uts, unix.AT_SYMLINK_NOFOLLOW)
}

func setCTimeNoFollow(path string, ctime time.Time) error {
	return nil
}
