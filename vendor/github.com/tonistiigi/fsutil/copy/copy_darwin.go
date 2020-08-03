// +build darwin

package fs

import (
	"io"
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// <sys/clonefile.h
// int clonefileat(int, const char *, int, const char *, uint32_t) __OSX_AVAILABLE(10.12) __IOS_AVAILABLE(10.0) __TVOS_AVAILABLE(10.0) __WATCHOS_AVAILABLE(3.0);

const CLONE_NOFOLLOW = 0x0001    /* Don't follow symbolic links */
const CLONE_NOOWNERCOPY = 0x0002 /* Don't copy ownership information from */

func copyFile(source, target string) error {
	if err := clonefile(source, target); err != nil {
		if err != unix.EINVAL {
			return err
		}
	} else {
		return nil
	}

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
	buf := bufferPool.Get().(*[]byte)
	_, err := io.CopyBuffer(dst, src, *buf)
	bufferPool.Put(buf)

	return err
}

// errnoErr returns common boxed Errno values, to prevent
// allocations at runtime.
func errnoErr(e syscall.Errno) error {
	switch e {
	case 0:
		return nil
	case unix.EAGAIN:
		return syscall.EAGAIN
	case unix.EINVAL:
		return syscall.EINVAL
	case unix.ENOENT:
		return syscall.ENOENT
	}
	return e
}

func clonefile(src, dst string) (err error) {
	var _p0, _p1 *byte
	_p0, err = unix.BytePtrFromString(src)
	if err != nil {
		return
	}
	_p1, err = unix.BytePtrFromString(dst)
	if err != nil {
		return
	}
	fdcwd := unix.AT_FDCWD
	_, _, e1 := unix.Syscall6(unix.SYS_CLONEFILEAT, uintptr(fdcwd), uintptr(unsafe.Pointer(_p0)), uintptr(fdcwd), uintptr(unsafe.Pointer(_p1)), uintptr(CLONE_NOFOLLOW), 0)
	if e1 != 0 {
		err = errnoErr(e1)
	}
	return
}
