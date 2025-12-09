//go:build unix && !tinygo

package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

const (
	nonBlockingFileReadSupported  = true
	nonBlockingFileWriteSupported = true
)

func rmdir(path string) sys.Errno {
	err := syscall.Rmdir(path)
	return sys.UnwrapOSError(err)
}

// readFd exposes syscall.Read.
func readFd(fd uintptr, buf []byte) (int, sys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // Short-circuit 0-len reads.
	}
	n, err := syscall.Read(int(fd), buf)
	errno := sys.UnwrapOSError(err)
	return n, errno
}

// writeFd exposes syscall.Write.
func writeFd(fd uintptr, buf []byte) (int, sys.Errno) {
	if len(buf) == 0 {
		return 0, 0 // Short-circuit 0-len writes.
	}
	n, err := syscall.Write(int(fd), buf)
	errno := sys.UnwrapOSError(err)
	return n, errno
}
