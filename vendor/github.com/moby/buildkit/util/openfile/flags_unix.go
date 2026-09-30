//go:build !linux && !windows

package openfile

import "syscall"

const (
	nonBlock = syscall.O_NONBLOCK
	noFollow = syscall.O_NOFOLLOW
)
