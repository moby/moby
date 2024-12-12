//go:build !windows

package archive

import (
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// checkSystemDriveAndRemoveDriveLetter is the non-Windows implementation
// of CheckSystemDriveAndRemoveDriveLetter
func checkSystemDriveAndRemoveDriveLetter(path string) (string, error) {
	return path, nil
}

// lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It returns a nil slice and nil error if the xattr is not set.
func lgetxattr(path string, attr string) ([]byte, error) {
	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, err := unix.Lgetxattr(path, attr, dest)

	for errors.Is(err, unix.ERANGE) {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, err = unix.Lgetxattr(path, attr, []byte{})
		if err != nil {
			return nil, err
		}
		dest = make([]byte, sz)
		sz, err = unix.Lgetxattr(path, attr, dest)
	}

	if err != nil {
		if errors.Is(err, unix.ENODATA) {
			return nil, nil
		}
		return nil, err
	}

	return dest[:sz], nil
}

// lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func lsetxattr(path string, attr string, data []byte, flags int) error {
	return unix.Lsetxattr(path, attr, data, flags)
}
