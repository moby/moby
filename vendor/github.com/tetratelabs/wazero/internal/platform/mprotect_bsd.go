//go:build (freebsd || netbsd || dragonfly) && !tinygo

package platform

import (
	"syscall"
	"unsafe"
)

// MprotectRX is like syscall.Mprotect with RX permission, defined locally so that BSD compiles.
func MprotectRX(b []byte) (err error) {
	var _p0 unsafe.Pointer
	if len(b) > 0 {
		_p0 = unsafe.Pointer(&b[0])
	}
	const prot = syscall.PROT_READ | syscall.PROT_EXEC
	_, _, e1 := syscall.Syscall(syscall.SYS_MPROTECT, uintptr(_p0), uintptr(len(b)), uintptr(prot))
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}
