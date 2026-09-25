//go:build !linux && !windows

package unix

import "syscall"

type Errno = syscall.Errno

// Errnos are distinct and non-zero.
const (
	E2BIG Errno = iota + 1
	EACCES
	EAGAIN
	EBADF
	EEXIST
	EFAULT
	EILSEQ
	EINTR
	EINVAL
	ENODEV
	ENOENT
	ENOSPC
	ENOTSUP
	ENOTSUPP
	EOPNOTSUPP
	EPERM
	ESRCH
	ESTALE
)
