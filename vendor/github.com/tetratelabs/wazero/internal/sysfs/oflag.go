package sysfs

import (
	"os"

	"github.com/tetratelabs/wazero/experimental/sys"
)

// toOsOpenFlag converts the input to the flag parameter of os.OpenFile
func toOsOpenFlag(oflag sys.Oflag) (flag int) {
	// First flags are exclusive
	switch oflag & (sys.O_RDONLY | sys.O_RDWR | sys.O_WRONLY) {
	case sys.O_RDONLY:
		flag |= os.O_RDONLY
	case sys.O_RDWR:
		flag |= os.O_RDWR
	case sys.O_WRONLY:
		flag |= os.O_WRONLY
	}

	// Run down the flags defined in the os package
	if oflag&sys.O_APPEND != 0 {
		flag |= os.O_APPEND
	}
	if oflag&sys.O_CREAT != 0 {
		flag |= os.O_CREATE
	}
	if oflag&sys.O_EXCL != 0 {
		flag |= os.O_EXCL
	}
	if oflag&sys.O_SYNC != 0 {
		flag |= os.O_SYNC
	}
	if oflag&sys.O_TRUNC != 0 {
		flag |= os.O_TRUNC
	}
	return withSyscallOflag(oflag, flag)
}
