package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

const supportedSyscallOflag = sys.O_DIRECTORY | sys.O_NOFOLLOW | sys.O_NONBLOCK

func withSyscallOflag(oflag sys.Oflag, flag int) int {
	if oflag&sys.O_DIRECTORY != 0 {
		flag |= syscall.O_DIRECTORY
	}
	// syscall.O_DSYNC not defined on darwin
	if oflag&sys.O_NOFOLLOW != 0 {
		flag |= syscall.O_NOFOLLOW
	}
	if oflag&sys.O_NONBLOCK != 0 {
		flag |= syscall.O_NONBLOCK
	}
	// syscall.O_RSYNC not defined on darwin
	return flag
}
