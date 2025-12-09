//go:build !windows && !plan9 && !tinygo

package sysfs

import (
	"syscall"

	"github.com/tetratelabs/wazero/experimental/sys"
)

func rename(from, to string) sys.Errno {
	if from == to {
		return 0
	}
	return sys.UnwrapOSError(syscall.Rename(from, to))
}
