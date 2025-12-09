//go:build !darwin && !linux && !windows && !illumos && !solaris && !freebsd

package sysfs

import (
	"github.com/tetratelabs/wazero/experimental/sys"
)

const supportedSyscallOflag = sys.Oflag(0)

func withSyscallOflag(oflag sys.Oflag, flag int) int {
	// O_DIRECTORY not defined
	// O_DSYNC not defined
	// O_NOFOLLOW not defined
	// O_NONBLOCK not defined
	// O_RSYNC not defined
	return flag
}
