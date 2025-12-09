//go:build !windows

package sys

func errorToErrno(err error) Errno {
	if errno, ok := err.(Errno); ok {
		return errno
	}
	if errno, ok := syscallToErrno(err); ok {
		return errno
	}
	return EIO
}
