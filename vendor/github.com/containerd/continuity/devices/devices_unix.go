// +build linux darwin freebsd solaris

package devices

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

func DeviceInfo(fi os.FileInfo) (uint64, uint64, error) {
	sys, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("cannot extract device from os.FileInfo")
	}

	dev := uint64(sys.Rdev)
	return uint64(unix.Major(dev)), uint64(unix.Minor(dev)), nil
}

// mknod provides a shortcut for syscall.Mknod
func Mknod(p string, mode os.FileMode, maj, min int) error {
	var (
		m   = syscallMode(mode.Perm())
		dev uint64
	)

	if mode&os.ModeDevice != 0 {
		dev = unix.Mkdev(uint32(maj), uint32(min))

		if mode&os.ModeCharDevice != 0 {
			m |= unix.S_IFCHR
		} else {
			m |= unix.S_IFBLK
		}
	} else if mode&os.ModeNamedPipe != 0 {
		m |= unix.S_IFIFO
	}

	return unix.Mknod(p, m, int(dev))
}

// syscallMode returns the syscall-specific mode bits from Go's portable mode bits.
func syscallMode(i os.FileMode) (o uint32) {
	o |= uint32(i.Perm())
	if i&os.ModeSetuid != 0 {
		o |= unix.S_ISUID
	}
	if i&os.ModeSetgid != 0 {
		o |= unix.S_ISGID
	}
	if i&os.ModeSticky != 0 {
		o |= unix.S_ISVTX
	}
	return
}
