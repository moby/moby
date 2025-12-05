//go:build (!windows && !linux && !darwin) || tinygo

package sysfs

import (
	"github.com/tetratelabs/wazero/experimental/sys"
)

func utimens(path string, atim, mtim int64) sys.Errno {
	return chtimes(path, atim, mtim)
}

func futimens(fd uintptr, atim, mtim int64) error {
	// Go exports syscall.Futimes, which is microsecond granularity, and
	// WASI tests expect nanosecond. We don't yet have a way to invoke the
	// futimens syscall portably.
	return sys.ENOSYS
}
