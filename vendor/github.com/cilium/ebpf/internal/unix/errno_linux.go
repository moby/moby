package unix

import (
	"syscall"

	linux "golang.org/x/sys/unix"
)

type Errno = syscall.Errno

const (
	E2BIG      = linux.E2BIG
	EACCES     = linux.EACCES
	EAGAIN     = linux.EAGAIN
	EBADF      = linux.EBADF
	EEXIST     = linux.EEXIST
	EFAULT     = linux.EFAULT
	EILSEQ     = linux.EILSEQ
	EINTR      = linux.EINTR
	EINVAL     = linux.EINVAL
	ENODEV     = linux.ENODEV
	ENOENT     = linux.ENOENT
	ENOSPC     = linux.ENOSPC
	EOPNOTSUPP = linux.EOPNOTSUPP
	EPERM      = linux.EPERM
	EPOLLIN    = linux.EPOLLIN
	ESRCH      = linux.ESRCH
	ESTALE     = linux.ESTALE
)
