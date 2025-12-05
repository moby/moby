//go:build !tinygo

package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

const supportedSyscallOflag = sys.O_DIRECTORY | sys.O_DSYNC | sys.O_NOFOLLOW | sys.O_NONBLOCK | sys.O_RSYNC

func withSyscallOflag(oflag sys.Oflag, flag int) int {
	if oflag&sys.O_DIRECTORY != 0 {
		flag |= syscall.O_DIRECTORY
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
