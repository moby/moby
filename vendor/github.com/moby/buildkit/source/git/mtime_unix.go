//go:build !windows

package git

import (
	"time"

	"golang.org/x/sys/unix"
)

func lchtimes(path string, t time.Time) error {
	ts := unix.NsecToTimespec(t.UnixNano())
	return unix.UtimesNanoAt(unix.AT_FDCWD, path, []unix.Timespec{ts, ts}, unix.AT_SYMLINK_NOFOLLOW)
}
