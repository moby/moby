//go:build !plan9 && !aix

package sys

import "syscall"

func syscallToErrno(err error) (Errno, bool) {
	errno, ok := err.(syscall.Errno)
	if !ok {
		return 0, false
	}
	switch errno {
	case 0:
		return 0, true
	case syscall.EACCES:
		return EACCES, true
	case syscall.EAGAIN:
		return EAGAIN, true
	case syscall.EBADF:
		return EBADF, true
	case syscall.EEXIST:
		return EEXIST, true
	case syscall.EFAULT:
		return EFAULT, true
	case syscall.EINTR:
		return EINTR, true
	case syscall.EINVAL:
		return EINVAL, true
	case syscall.EIO:
		return EIO, true
	case syscall.EISDIR:
		return EISDIR, true
	case syscall.ELOOP:
		return ELOOP, true
	case syscall.ENAMETOOLONG:
		return ENAMETOOLONG, true
	case syscall.ENOENT:
		return ENOENT, true
	case syscall.ENOSYS:
		return ENOSYS, true
	case syscall.ENOTDIR:
		return ENOTDIR, true
	case syscall.ERANGE:
		return ERANGE, true
	case syscall.ENOTEMPTY:
		return ENOTEMPTY, true
	case syscall.ENOTSOCK:
		return ENOTSOCK, true
	case syscall.ENOTSUP:
		return ENOTSUP, true
	case syscall.EPERM:
		return EPERM, true
	case syscall.EROFS:
		return EROFS, true
	default:
		return EIO, true
	}
}

// Unwrap is a convenience for runtime.GOOS which define syscall.Errno.
func (e Errno) Unwrap() error {
	switch e {
	case 0:
		return nil
	case EACCES:
		return syscall.EACCES
	case EAGAIN:
		return syscall.EAGAIN
	case EBADF:
		return syscall.EBADF
	case EEXIST:
		return syscall.EEXIST
	case EFAULT:
		return syscall.EFAULT
	case EINTR:
		return syscall.EINTR
	case EINVAL:
		return syscall.EINVAL
	case EIO:
		return syscall.EIO
	case EISDIR:
		return syscall.EISDIR
	case ELOOP:
		return syscall.ELOOP
	case ENAMETOOLONG:
		return syscall.ENAMETOOLONG
	case ENOENT:
		return syscall.ENOENT
	case ENOSYS:
		return syscall.ENOSYS
	case ENOTDIR:
		return syscall.ENOTDIR
	case ENOTEMPTY:
		return syscall.ENOTEMPTY
	case ENOTSOCK:
		return syscall.ENOTSOCK
	case ENOTSUP:
		return syscall.ENOTSUP
	case EPERM:
		return syscall.EPERM
	case EROFS:
		return syscall.EROFS
	default:
		return syscall.EIO
	}
}
