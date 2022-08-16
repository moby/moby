//go:build solaris || darwin || freebsd
// +build solaris darwin freebsd

package fs

import (
	"os"
	"syscall"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func getUIDGID(fi os.FileInfo) (uid, gid int) {
	st := fi.Sys().(*syscall.Stat_t)
	return int(st.Uid), int(st.Gid)
}

func (c *copier) copyFileInfo(fi os.FileInfo, name string) error {
	st := fi.Sys().(*syscall.Stat_t)
	chown := c.chown
	uid, gid := getUIDGID(fi)
	old := &User{UID: uid, GID: gid}
	if chown == nil {
		chown = func(u *User) (*User, error) {
			return u, nil
		}
	}
	if err := Chown(name, old, chown); err != nil {
		return errors.Wrapf(err, "failed to chown %s", name)
	}

	m := fi.Mode()
	if c.mode != nil {
		m = (m & ^os.FileMode(0777)) | os.FileMode(*c.mode&0777)
	}
	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		if err := os.Chmod(name, m); err != nil {
			return errors.Wrapf(err, "failed to chmod %s", name)
		}
	}

	if c.utime != nil {
		if err := Utimes(name, c.utime); err != nil {
			return err
		}
	} else {
		timespec := []unix.Timespec{unix.Timespec(StatAtime(st)), unix.Timespec(StatMtime(st))}
		if err := unix.UtimesNanoAt(unix.AT_FDCWD, name, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return errors.Wrapf(err, "failed to utime %s", name)
		}
	}
	return nil
}
