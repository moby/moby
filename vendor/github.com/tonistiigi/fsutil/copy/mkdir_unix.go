//go:build !windows
// +build !windows

package fs

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func fixRootDirectory(p string) string {
	return p
}

func Utimes(p string, tm *time.Time) error {
	if tm == nil {
		return nil
	}

	ts, err := unix.TimeToTimespec(*tm)
	if err != nil {
		return err
	}

	timespec := []unix.Timespec{ts, ts}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, p, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return errors.Wrapf(err, "failed to utime %s", p)
	}

	return nil
}

func Chown(p string, old *User, fn Chowner) error {
	if fn == nil {
		return nil
	}
	user, err := fn(old)
	if err != nil {
		return errors.WithStack(err)
	}
	if user != nil {
		if err := os.Lchown(p, user.UID, user.GID); err != nil {
			return err
		}
	}
	return nil
}
