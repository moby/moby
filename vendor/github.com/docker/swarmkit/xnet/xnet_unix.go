// +build !windows

package xnet

import (
	"net"
	"time"
)

// ListenLocal opens a local socket for control communication
func ListenLocal(socket string) (net.Listener, error) {
	// on unix it's just a unix socket
	return net.Listen("unix", socket)
}

// DialTimeoutLocal is a DialTimeout function for local sockets
func DialTimeoutLocal(socket string, timeout time.Duration) (net.Conn, error) {
	// on unix, we dial a unix socket
	return net.DialTimeout("unix", socket, timeout)
}
