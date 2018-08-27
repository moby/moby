// +build !linux,!darwin

package sysx

import (
	"errors"
	"runtime"
)

var unsupported = errors.New("extended attributes unsupported on " + runtime.GOOS)

// Listxattr calls syscall listxattr and reads all content
// and returns a string array
func Listxattr(path string) ([]string, error) {
	return []string{}, nil
}

// Removexattr calls syscall removexattr
func Removexattr(path string, attr string) (err error) {
	return unsupported
}

// Setxattr calls syscall setxattr
func Setxattr(path string, attr string, data []byte, flags int) (err error) {
	return unsupported
}

// Getxattr calls syscall getxattr
func Getxattr(path, attr string) ([]byte, error) {
	return []byte{}, unsupported
}

// LListxattr lists xattrs, not following symlinks
func LListxattr(path string) ([]string, error) {
	return []string{}, nil
}

// LRemovexattr removes an xattr, not following symlinks
func LRemovexattr(path string, attr string) (err error) {
	return unsupported
}

// LSetxattr sets an xattr, not following symlinks
func LSetxattr(path string, attr string, data []byte, flags int) (err error) {
	return unsupported
}

// LGetxattr gets an xattr, not following symlinks
func LGetxattr(path, attr string) ([]byte, error) {
	return []byte{}, nil
}
