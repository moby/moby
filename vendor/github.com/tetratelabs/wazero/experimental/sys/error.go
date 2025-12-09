package sys

import (
	"io"
	"io/fs"
	"os"
)

// UnwrapOSError returns an Errno or zero if the input is nil.
func UnwrapOSError(err error) Errno {
	if err == nil {
		return 0
	}
	err = underlyingError(err)
	switch err {
	case nil, io.EOF:
		return 0 // EOF is not a Errno
	case fs.ErrInvalid:
		return EINVAL
	case fs.ErrPermission:
		return EPERM
	case fs.ErrExist:
		return EEXIST
	case fs.ErrNotExist:
		return ENOENT
	case fs.ErrClosed:
		return EBADF
	}
	return errorToErrno(err)
}

// underlyingError returns the underlying error if a well-known OS error type.
//
// This impl is basically the same as os.underlyingError in os/error.go
func underlyingError(err error) error {
	switch err := err.(type) {
	case *os.PathError:
		return err.Err
	case *os.LinkError:
		return err.Err
	case *os.SyscallError:
		return err.Err
	}
	return err
}
