package sockets

import (
	"os"
	"strconv"
	"strings"
	"syscall"
)

// maxListenerBacklog returns the maximum length of the queue of pending
// connections for a listening socket.
//
// It is similar to in stdlib, but without the fallbacks for Kernel < 4.1.0;
// https://github.com/golang/go/blob/go1.26.3/src/net/sock_linux.go#L33-L53
func maxListenerBacklog() int {
	b, err := os.ReadFile("/proc/sys/net/core/somaxconn")
	if err != nil {
		return syscall.SOMAXCONN
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return syscall.SOMAXCONN
	}
	return n
}
