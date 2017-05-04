// +build darwin freebsd

package sysx

import (
	"syscall"
)

const ENODATA = syscall.ENOATTR
