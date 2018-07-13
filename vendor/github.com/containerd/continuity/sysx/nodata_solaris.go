package sysx

import (
	"syscall"
)

// This should actually be a set that contains ENOENT and EPERM
const ENODATA = syscall.ENOENT
