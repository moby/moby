// +build linux

package system // import "github.com/docker/docker/pkg/system"

import "golang.org/x/sys/unix"

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It will returns a nil slice and nil error if the xattr is not set.
// It doesn't follow symlinks.
func Lgetxattr(path string, attr string) ([]byte, error) {
	// Start with a 128 length byte array
	dest := make([]byte, 128)
	sz, errno := unix.Lgetxattr(path, attr, dest)

	for errno == unix.ERANGE {
		// Buffer too small, use zero-sized buffer to get the actual size
		sz, errno = unix.Lgetxattr(path, attr, []byte{})
		if errno != nil {
			return nil, errno
		}
		dest = make([]byte, sz)
		sz, errno = unix.Lgetxattr(path, attr, dest)
	}

	switch {
	case errno == unix.ENODATA:
		return nil, nil
	case errno != nil:
		return nil, errno
	}

	return dest[:sz], nil
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It doesn't follow symlinks.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	return unix.Lsetxattr(path, attr, data, flags)
}

// Llistxattr lists extended attributes associated with the given path in the
// file system. It doesn't follow symlinks.
func Llistxattr(path string) ([]string, error) {
	size, err := unix.Llistxattr(path, nil)
	if err != nil {
		if err == unix.ENOTSUP || err == unix.EOPNOTSUPP {
			// filesystem does not support extended attributes
			return nil, nil
		}
		return nil, err
	}
	if size <= 0 {
		return nil, nil
	}

	buf := make([]byte, size)
	read, err := unix.Llistxattr(path, buf)
	if err != nil {
		return nil, err
	}

	return stringsFromByteSlice(buf[:read]), nil
}

// stringsFromByteSlice converts a sequence of attributes to a []string.
func stringsFromByteSlice(buf []byte) []string {
	var result []string
	off := 0
	for i, b := range buf {
		if b == 0 {
			result = append(result, string(buf[off:i]))
			off = i + 1
		}
	}
	return result
}
