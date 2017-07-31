package system

import "golang.org/x/sys/unix"

// Lgetxattr retrieves the value of the extended attribute identified by attr
// and associated with the given path in the file system.
// It will returns a nil slice and nil error if the xattr is not set.
func Lgetxattr(path string, attr string) ([]byte, error) {
	dest := make([]byte, 128)
	sz, errno := unix.Lgetxattr(path, attr, dest)
	if errno == unix.ENODATA {
		return nil, nil
	}
	if errno == unix.ERANGE {
		dest = make([]byte, sz)
		sz, errno = unix.Lgetxattr(path, attr, dest)
	}
	if errno != nil {
		return nil, errno
	}

	return dest[:sz], nil
}

// Lsetxattr sets the value of the extended attribute identified by attr
// and associated with the given path in the file system.
func Lsetxattr(path string, attr string, data []byte, flags int) error {
	return unix.Lsetxattr(path, attr, data, flags)
}

// Listxattr retrieves a list of names of extened attributes associated
// with the given path in the file system
func Listxattr(path string) ([]string, error) {
	// Get the size first
	size, err := unix.Listxattr(path, nil)
	if err != nil {
		return nil, err
	}
	if size > 0 {
		buf := make([]byte, size)
		// Read into buffer of that size.
		read, err := unix.Listxattr(path, buf)
		if err != nil {
			return nil, err
		}
		return nullTermToStrings(buf[:read]), nil
	}
	return []string{}, nil
}

// nullTermToStrings converts an array of NULL terminated UTF-8 strings to a []string.
func nullTermToStrings(buf []byte) (result []string) {
	offset := 0
	for index, b := range buf {
		if b == 0 {
			result = append(result, string(buf[offset:index]))
			offset = index + 1
		}
	}
	return
}
