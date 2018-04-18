// +build !windows

package syscallx

import "syscall"

// Readlink returns the destination of the named symbolic link.
func Readlink(path string, buf []byte) (n int, err error) {
	return syscall.Readlink(path, buf)
}
