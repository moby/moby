//go:build solaris && !tinygo

package platform

import "syscall"

func MprotectRX(b []byte) error {
	return syscall.ENOTSUP
}
