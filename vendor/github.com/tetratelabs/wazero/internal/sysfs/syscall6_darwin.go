package sysfs

import (
	"syscall"
	_ "unsafe"
)

// syscall_syscall6 is a private symbol that we link below. We need to use this
// instead of syscall.Syscall6 because the public syscall.Syscall6 won't work
// when fn is an address.
//
//go:linkname syscall_syscall6 syscall.syscall6
func syscall_syscall6(fn, a1, a2, a3, a4, a5, a6 uintptr) (r1, r2 uintptr, err syscall.Errno)
