// +build linux darwin freebsd solaris

package devices

import (
	"fmt"
	"os"
	"syscall"
)

func DeviceInfo(fi os.FileInfo) (uint64, uint64, error) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("cannot extract device from os.FileInfo")
	}

	return getmajor(sys.Rdev), getminor(sys.Rdev), nil
}

// mknod provides a shortcut for syscall.Mknod
func Mknod(p string, mode os.FileMode, maj, min int) error {
	var (
		m   = syscallMode(mode.Perm())
		dev int
	)

	if mode&os.ModeDevice != 0 {
		dev = makedev(maj, min)

		if mode&os.ModeCharDevice != 0 {
			m |= syscall.S_IFCHR
		} else {
			m |= syscall.S_IFBLK
		}
	} else if mode&os.ModeNamedPipe != 0 {
		m |= syscall.S_IFIFO
	}

	return syscall.Mknod(p, m, dev)
}

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= syscall.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= syscall.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= syscall.S_ISVTX
	}
	return
}
