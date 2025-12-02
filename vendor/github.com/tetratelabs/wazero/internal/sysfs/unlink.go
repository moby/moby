//go:build !windows && !plan9 && !tinygo

package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func unlink(name string) (errno sys.Errno) {
	err := syscall.Unlink(name)
	if errno = sys.UnwrapOSError(err); errno == sys.EPERM {
		errno = sys.EISDIR
	}
	return errno
}
