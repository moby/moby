//go:build !linux && !darwin && !dragonfly && !freebsd && !netbsd && !openbsd && !windows

package sockets

import "syscall"

func maxListenerBacklog() int {
	return syscall.SOMAXCONN
}
