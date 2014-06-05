package system

import (
	"syscall"
	"unsafe"
)

var _zero uintptr

func Llistxattr(path string, dest []byte) (size int, err error) {
	pathBytes, err := syscall.BytePtrFromString(path)

	if err != nil {
		return -1, err
	}
	var newpathBytes unsafe.Pointer

	if len(dest) > 0 {
		newpathBytes = unsafe.Pointer(&dest[0])
	} else {
		newpathBytes = unsafe.Pointer(&_zero)
	}

	_size, _, errno := syscall.Syscall6(syscall.SYS_LLISTXATTR, uintptr(unsafe.Pointer(pathBytes)), uintptr(newpathBytes), uintptr(len(dest)), 0, 0, 0)
	size = int(_size)
	if errno != 0 {
		return -1, errno
	}

	return size, nil
}

// Returns a nil slice and nil error if the xattr is not set
func Lgetxattr(path string, attr string) ([]byte, error) {
	pathBytes, err := syscall.BytePtrFromString(path)
	if err != nil {
		return nil, err
	}
	attrBytes, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return nil, err
	}

	dest := make([]byte, 128)
	destBytes := unsafe.Pointer(&dest[0])
	sz, _, errno := syscall.Syscall6(syscall.SYS_LGETXATTR, uintptr(unsafe.Pointer(pathBytes)), uintptr(unsafe.Pointer(attrBytes)), uintptr(destBytes), uintptr(len(dest)), 0, 0)

	switch {
	case errno == syscall.ENODATA:
		return nil, errno
	case errno == syscall.ENOTSUP:
		return nil, errno
	case errno == syscall.ERANGE:
		dest = make([]byte, sz)
		destBytes := unsafe.Pointer(&dest[0])
		sz, _, errno = syscall.Syscall6(syscall.SYS_LGETXATTR, uintptr(unsafe.Pointer(pathBytes)), uintptr(unsafe.Pointer(attrBytes)), uintptr(destBytes), uintptr(len(dest)), 0, 0)
		if errno != 0 {
			return nil, errno
		}
	case errno != 0:
		return nil, errno
	}
	return dest[:sz], nil
}

func Lsetxattr(path string, attr string, data []byte, flags int) error {
	pathBytes, err := syscall.BytePtrFromString(path)
	if err != nil {
		return err
	}
	attrBytes, err := syscall.BytePtrFromString(attr)
	if err != nil {
		return err
	}
	var dataBytes unsafe.Pointer
	if len(data) > 0 {
		dataBytes = unsafe.Pointer(&data[0])
	} else {
		dataBytes = unsafe.Pointer(&_zero)
	}
	_, _, errno := syscall.Syscall6(syscall.SYS_LSETXATTR, uintptr(unsafe.Pointer(pathBytes)), uintptr(unsafe.Pointer(attrBytes)), uintptr(dataBytes), uintptr(len(data)), uintptr(flags), 0)
	if errno != 0 {
		return errno
	}
	return nil
}
