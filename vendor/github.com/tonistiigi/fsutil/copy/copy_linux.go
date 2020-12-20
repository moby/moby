package fs

import (
	"io"
	"math"
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

func copyFile(source, target string) error {
	src, err := os.Open(source)
	if err != nil {
		return errors.Wrapf(err, "failed to open source %s", source)
	}
	defer src.Close()
	tgt, err := os.Create(target)
	if err != nil {
		return errors.Wrapf(err, "failed to open target %s", target)
	}
	defer tgt.Close()

	return copyFileContent(tgt, src)
}

func copyFileContent(dst, src *os.File) error {
	st, err := src.Stat()
	if err != nil {
		return errors.Wrap(err, "unable to stat source")
	}

	var written int64
	size := st.Size()
	first := true

	for written < size {
		var desired int
		if size-written > math.MaxInt32 {
			desired = int(math.MaxInt32)
		} else {
			desired = int(size - written)
		}

		n, err := unix.CopyFileRange(int(src.Fd()), nil, int(dst.Fd()), nil, desired, 0)
		if err != nil {
			if (err != unix.ENOSYS && err != unix.EXDEV && err != unix.EPERM) || !first {
				return errors.Wrap(err, "copy file range failed")
			}

			buf := bufferPool.Get().(*[]byte)
			_, err = io.CopyBuffer(dst, src, *buf)
			bufferPool.Put(buf)
			return errors.Wrap(err, "userspace copy failed")
		}

		first = false
		written += int64(n)
	}
	return nil
}

func copyDevice(dst string, fi os.FileInfo) error {
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return errors.New("unsupported stat type")
	}
	return unix.Mknod(dst, uint32(fi.Mode()), int(st.Rdev))
}
