//go:build !(unix || windows) || tinygo

package sysfs

import (
	"os"

	"github.com/tetratelabs/wazero/experimental/sys"
)

const (
	nonBlockingFileReadSupported  = false
	nonBlockingFileWriteSupported = false
)

func rmdir(path string) sys.Errno {
	return sys.UnwrapOSError(os.Remove(path))
}

// readFd returns ENOSYS on unsupported platforms.
func readFd(fd uintptr, buf []byte) (int, sys.Errno) {
	return -1, sys.ENOSYS
}

// writeFd returns ENOSYS on unsupported platforms.
func writeFd(fd uintptr, buf []byte) (int, sys.Errno) {
	return -1, sys.ENOSYS
}
