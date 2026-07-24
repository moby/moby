//go:build !linux && !windows

package sockets

import (
	"runtime"
	"syscall"
)

// maxListenerBacklog is similar to the equivalent in stdlib;
// https://github.com/golang/go/blob/go1.26.3/src/net/sock_bsd.go#L14-L39
func maxListenerBacklog() int {
	var (
		n   uint32
		err error
	)
	switch runtime.GOOS {
	case "darwin", "ios":
		n, err = syscall.SysctlUint32("kern.ipc.somaxconn")
	case "freebsd":
		n, err = syscall.SysctlUint32("kern.ipc.soacceptqueue")
	case "netbsd":
		// NOTE: NetBSD has no somaxconn-like kernel state so far
	case "openbsd":
		n, err = syscall.SysctlUint32("kern.somaxconn")
	default:
		return syscall.SOMAXCONN
	}
	if n == 0 || err != nil {
		return syscall.SOMAXCONN
	}
	// FreeBSD stores the backlog in a uint16, as does Linux.
	// Assume the other BSDs do too. Truncate number to avoid wrapping.
	// See issue 5030.
	if n > 1<<16-1 {
		n = 1<<16 - 1
	}
	return int(n)
}
