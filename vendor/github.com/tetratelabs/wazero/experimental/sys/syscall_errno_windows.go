package sys

import "golang.org/x/sys/windows"

func errorToErrno(err error) Errno {
	switch err := err.(type) {
	case Errno:
		return err
	case windows.Errno:
		// Note: In windows, _ERROR_PATH_NOT_FOUND(0x3) maps to syscall.ENOTDIR
		switch err {
		case windows.ERROR_ALREADY_EXISTS, windows.ERROR_FILE_EXISTS:
			return EEXIST
		case windows.ERROR_DIRECTORY:
			// ERROR_DIRECTORY is returned by syscall.Rmdir.
			return ENOTDIR
		case windows.ERROR_DIR_NOT_EMPTY:
			return ENOTEMPTY
		case windows.ERROR_INVALID_HANDLE, windows.WSAENOTSOCK, windows.ERROR_ACCESS_DENIED:
			// WSAENOTSOCK is returned by winsock_select when a given handle is not a socket.
			// POSIX read and write functions expect EBADF, not EACCES when not
			// open for reading or writing.
			return EBADF
		case windows.ERROR_PRIVILEGE_NOT_HELD:
			return EPERM
		case windows.ERROR_NEGATIVE_SEEK, windows.ERROR_NOT_A_REPARSE_POINT, windows.ERROR_INVALID_NAME:
			// ERROR_NEGATIVE_SEEK is returned by os.Truncate.
			// ERROR_NOT_A_REPARSE_POINT is returned by os.Readlink.
			// ERROR_INVALID_NAME is returned by open when a file path has a trailing slash.
			return EINVAL
		}
		errno, _ := syscallToErrno(err)
		return errno
	default:
		return EIO
	}
}
