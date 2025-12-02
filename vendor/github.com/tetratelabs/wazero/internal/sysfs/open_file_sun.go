//go:build illumos || solaris

package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

const supportedSyscallOflag = sys.O_DIRECTORY | sys.O_DSYNC | sys.O_NOFOLLOW | sys.O_NONBLOCK | sys.O_RSYNC

func withSyscallOflag(oflag sys.Oflag, flag int) int {
	if oflag&sys.O_DIRECTORY != 0 {
		// See https://github.com/illumos/illumos-gate/blob/edd580643f2cf1434e252cd7779e83182ea84945/usr/src/uts/common/sys/fcntl.h#L90
		flag |= 0x1000000
	}
	if oflag&sys.O_DSYNC != 0 {
		flag |= syscall.O_DSYNC
	}
	if oflag&sys.O_NOFOLLOW != 0 {
		flag |= syscall.O_NOFOLLOW
	}
	if oflag&sys.O_NONBLOCK != 0 {
		flag |= syscall.O_NONBLOCK
	}
	if oflag&sys.O_RSYNC != 0 {
		flag |= syscall.O_RSYNC
	}
	return flag
}
