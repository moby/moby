package sys

import "strconv"

// Errno is a subset of POSIX errno used by wazero interfaces. Zero is not an
// error. Other values should not be interpreted numerically, rather by constants
// prefixed with 'E'.
//
// See https://pubs.opengroup.org/onlinepubs/9699919799/basedefs/errno.h.html
type Errno uint16

// ^-- Note: This will eventually move to the public /sys package. It is
// experimental until we audit the socket related APIs to ensure we have all
// the Errno it returns, and we export fs.FS. This is not in /internal/sys as
// that would introduce a package cycle.

// This is a subset of errors to reduce implementation burden. `wasip1` defines
// almost all POSIX error numbers, but not all are used in practice. wazero
// will add ones needed in POSIX order, as needed by functions that explicitly
// document returning them.
//
// https://github.com/WebAssembly/WASI/blob/snapshot-01/phases/snapshot/docs.md#-errno-enumu16
const (
	EACCES Errno = iota + 1
	EAGAIN
	EBADF
	EEXIST
	EFAULT
	EINTR
	EINVAL
	EIO
	EISDIR
	ELOOP
	ENAMETOOLONG
	ENOENT
	ENOSYS
	ENOTDIR
	ERANGE
	ENOTEMPTY
	ENOTSOCK
	ENOTSUP
	EPERM
	EROFS

	// NOTE ENOTCAPABLE is defined in wasip1, but not in POSIX. wasi-libc
	// converts it to EBADF, ESPIPE or EINVAL depending on the call site.
	// It isn't known if compilers who don't use ENOTCAPABLE would crash on it.
)

// Error implements error
func (e Errno) Error() string {
	switch e {
	case 0: // not an error
		return "success"
	case EACCES:
		return "permission denied"
	case EAGAIN:
		return "resource unavailable, try again"
	case EBADF:
		return "bad file descriptor"
	case EEXIST:
		return "file exists"
	case EFAULT:
		return "bad address"
	case EINTR:
		return "interrupted function"
	case EINVAL:
		return "invalid argument"
	case EIO:
		return "input/output error"
	case EISDIR:
		return "is a directory"
	case ELOOP:
		return "too many levels of symbolic links"
	case ENAMETOOLONG:
		return "filename too long"
	case ENOENT:
		return "no such file or directory"
	case ENOSYS:
		return "functionality not supported"
	case ENOTDIR:
		return "not a directory or a symbolic link to a directory"
	case ERANGE:
		return "result too large"
	case ENOTEMPTY:
		return "directory not empty"
	case ENOTSOCK:
		return "not a socket"
	case ENOTSUP:
		return "not supported (may be the same value as [EOPNOTSUPP])"
	case EPERM:
		return "operation not permitted"
	case EROFS:
		return "read-only file system"
	default:
		return "Errno(" + strconv.Itoa(int(e)) + ")"
	}
}
