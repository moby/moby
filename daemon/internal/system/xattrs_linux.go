package system

import (
	"errors"

	"golang.org/x/sys/unix"
)

type XattrError struct {
	Op   string
	Attr string
	Path string
	Err  error
}

func (e *XattrError) Error() string { return e.Op + " " + e.Attr + " " + e.Path + ": " + e.Err.Error() }

func (e *XattrError) Unwrap() error { return e.Err }

// Timeout reports whether this error represents a timeout.
func (e *XattrError) Timeout() bool {
	t, ok := e.Err.(interface{ Timeout() bool })
	return ok && t.Timeout()
}

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It returns a nil slice and nil error if the xattr is not set.
func Lgetxattr(path string, attr string) ([]byte, error) {
	sysErr := func(err error) ([]byte, error) {
		return nil, &XattrError{Op: "lgetxattr", Attr: attr, Path: path, Err: err}
	}

	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, errno := unix.Lgetxattr(path, attr, dest)

	for errors.Is(errno, unix.ERANGE) {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = unix.Lgetxattr(path, attr, []byte{})
		if errno != nil {
			return sysErr(errno)
		}
		dest = make([]byte, sz)
		sz, errno = unix.Lgetxattr(path, attr, dest)
	}

	switch {
	case errors.Is(errno, unix.ENODATA):
		return nil, nil
	case errno != nil:
		return sysErr(errno)
	}

	return dest[:sz], nil
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	err := unix.Lsetxattr(path, attr, data, flags)
	if err != nil {
		return &XattrError{Op: "lsetxattr", Attr: attr, Path: path, Err: err}
	}
	return nil
}
