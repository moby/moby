package unix

// The code in this file is derived from syscall_unix.go in the Go source code,
// licensed under the MIT license.

import (
	"errors"
	"os"
	"syscall"
)

//go:generate go run golang.org/x/tools/cmd/stringer@latest -type=Errno -tags=windows -output=errno_string_windows.go

// Windows specific constants for Unix errnos.
//
// The values do not always match Linux, for example EILSEQ and EOPNOTSUPP.
//
// See https://learn.microsoft.com/en-us/cpp/c-runtime-library/errno-constants?view=msvc-170
const (
	EPERM       Errno = 1
	ENOENT      Errno = 2
	ESRCH       Errno = 3
	EINTR       Errno = 4
	E2BIG       Errno = 7
	EBADF       Errno = 9
	EAGAIN      Errno = 11
	EACCES      Errno = 13
	EFAULT      Errno = 14
	EEXIST      Errno = 17
	ENODEV      Errno = 19
	EINVAL      Errno = 22
	ENFILE      Errno = 23
	EMFILE      Errno = 24
	ENOSPC      Errno = 28
	ENOSYS      Errno = 40
	ENOTEMPTY   Errno = 41
	EILSEQ      Errno = 42
	ENOTSUP     Errno = 129
	EOPNOTSUPP  Errno = 130
	ETIMEDOUT   Errno = 138
	EWOULDBLOCK Errno = 140
)

// These constants do not exist on Windows and therefore have a non-zero
// dummy value.
const (
	ENOTSUPP Errno = Errno(syscall.APPLICATION_ERROR) + iota
	ESTALE
)

// Errno is a Windows compatibility shim for Unix errnos.
type Errno uintptr

func (e Errno) Error() string {
	return e.String()
}

func (e Errno) Is(target error) bool {
	switch target {
	case os.ErrPermission:
		return e == EACCES || e == EPERM
	case os.ErrExist:
		return e == EEXIST || e == ENOTEMPTY
	case os.ErrNotExist:
		return e == ENOENT
	case errors.ErrUnsupported:
		return e == ENOSYS || e == ENOTSUP || e == EOPNOTSUPP
	}
	return false
}

func (e Errno) Temporary() bool {
	return e == EINTR || e == EMFILE || e == ENFILE || e.Timeout()
}

func (e Errno) Timeout() bool {
	return e == EAGAIN || e == EWOULDBLOCK || e == ETIMEDOUT
}
