//go:build plan9 || tinygo

package sysfs

import "github.com/tetratelabs/wazero/experimental/sys"

func setNonblock(fd uintptr, enable bool) sys.Errno {
	return sys.ENOSYS
}

func isNonblock(f *osFile) bool {
	return false
}
