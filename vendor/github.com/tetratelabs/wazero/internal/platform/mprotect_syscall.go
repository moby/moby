//go:build (linux || darwin) && !tinygo

package platform

import "syscall"

// MprotectRX is like syscall.Mprotect with RX permission.
func MprotectRX(b []byte) (err error) {
	return syscall.Mprotect(b, syscall.PROT_READ|syscall.PROT_EXEC)
}
