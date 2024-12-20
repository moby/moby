//go:build solaris || darwin || freebsd || openbsd || netbsd
// +build solaris darwin freebsd openbsd netbsd

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

func (c *copier) copyFileInfo(fi os.FileInfo, src, name string) error {
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
	if c.modeSet != nil {
		m = c.modeSet.Apply(m)
	} else if c.mode != nil {
		m = os.FileMode(*c.mode).Perm()
		if *c.mode&syscall.S_ISGID != 0 {
			m |= os.ModeSetgid
		}
		if *c.mode&syscall.S_ISUID != 0 {
			m |= os.ModeSetuid
		}
		if *c.mode&syscall.S_ISVTX != 0 {
			m |= os.ModeSticky
		}
	}
	if (fi.Mode() & os.ModeSymlink) != os.ModeSymlink {
		if err := os.Chmod(name, m); err != nil {
			return errors.Wrapf(err, "failed to chmod %s", name)
		}
	}

	if err := c.copyFileTimestamp(fi, name); err != nil {
		return err
	}
	return nil
}

func (c *copier) copyFileTimestamp(fi os.FileInfo, name string) error {
	if c.utime != nil {
		return Utimes(name, c.utime)
	}

	st := fi.Sys().(*syscall.Stat_t)
	timespec := []unix.Timespec{unix.Timespec(StatAtime(st)), unix.Timespec(StatMtime(st))}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, name, timespec, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return errors.Wrapf(err, "failed to utime %s", name)
	}
	return nil
}
