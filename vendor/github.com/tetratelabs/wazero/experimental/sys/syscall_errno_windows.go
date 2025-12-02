package sys

import "syscall"

// These are errors not defined in the syscall package. They are prefixed with
// underscore to avoid exporting them.
//
// See https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--0-499-
const (
	// _ERROR_INVALID_HANDLE is a Windows error returned by syscall.Write
	// instead of syscall.EBADF
	_ERROR_INVALID_HANDLE = syscall.Errno(6)

	// _ERROR_INVALID_NAME is a Windows error returned by open when a file
	// path has a trailing slash
	_ERROR_INVALID_NAME = syscall.Errno(0x7B)

	// _ERROR_NEGATIVE_SEEK is a Windows error returned by os.Truncate
	// instead of syscall.EINVAL
	_ERROR_NEGATIVE_SEEK = syscall.Errno(0x83)

	// _ERROR_DIRECTORY is a Windows error returned by syscall.Rmdir
	// instead of syscall.ENOTDIR
	_ERROR_DIRECTORY = syscall.Errno(0x10B)

	// _ERROR_NOT_A_REPARSE_POINT is a Windows error returned by os.Readlink
	// instead of syscall.EINVAL
	_ERROR_NOT_A_REPARSE_POINT = syscall.Errno(0x1126)

	// _ERROR_INVALID_SOCKET is a Windows error returned by winsock_select
	// when a given handle is not a socket.
	_ERROR_INVALID_SOCKET = syscall.Errno(0x2736)
)

func errorToErrno(err error) Errno {
	switch err := err.(type) {
	case Errno:
		return err
	case syscall.Errno:
		// Note: In windows, _ERROR_PATH_NOT_FOUND(0x3) maps to syscall.ENOTDIR
		switch err {
		case syscall.ERROR_ALREADY_EXISTS:
			return EEXIST
		case _ERROR_DIRECTORY:
			return ENOTDIR
		case syscall.ERROR_DIR_NOT_EMPTY:
			return ENOTEMPTY
		case syscall.ERROR_FILE_EXISTS:
			return EEXIST
		case _ERROR_INVALID_HANDLE, _ERROR_INVALID_SOCKET:
			return EBADF
		case syscall.ERROR_ACCESS_DENIED:
			// POSIX read and write functions expect EBADF, not EACCES when not
			// open for reading or writing.
			return EBADF
		case syscall.ERROR_PRIVILEGE_NOT_HELD:
			return EPERM
		case _ERROR_NEGATIVE_SEEK, _ERROR_INVALID_NAME, _ERROR_NOT_A_REPARSE_POINT:
			return EINVAL
		}
		errno, _ := syscallToErrno(err)
		return errno
	default:
		return EIO
	}
}
