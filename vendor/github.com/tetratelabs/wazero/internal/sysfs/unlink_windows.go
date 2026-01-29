package sysfs

import (
	"os"
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func unlink(name string) sys.Errno {
	err := syscall.Unlink(name)
	if err == nil {
		return 0
	}
	errno := sys.UnwrapOSError(err)
	if errno == sys.EBADF {
		lstat, errLstat := os.Lstat(name)
		if errLstat == nil && lstat.Mode()&os.ModeSymlink != 0 {
			errno = sys.UnwrapOSError(os.Remove(name))
		} else {
			errno = sys.EISDIR
		}
	}
	return errno
}
