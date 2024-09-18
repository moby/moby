//go:build aix || darwin || dragonfly || freebsd || linux || netbsd || openbsd
// +build aix darwin dragonfly freebsd linux netbsd openbsd

package dns

import (
	"context"
	"net"
	"syscall"

	"golang.org/x/sys/unix"
)

const supportsReusePort = true

func reuseportControl(network, address string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
	})
	if err != nil {
		return err
	}

	return opErr
}

const supportsReuseAddr = true

func reuseaddrControl(network, address string, c syscall.RawConn) error {
	var opErr error
	err := c.Control(func(fd uintptr) {
		opErr = unix.SetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_REUSEADDR, 1)
	})
	if err != nil {
		return err
	}

	return opErr
}

func listenTCP(network, addr string, reuseport, reuseaddr bool) (net.Listener, error) {
	var lc net.ListenConfig
	switch {
	case reuseaddr && reuseport:
	case reuseport:
		lc.Control = reuseportControl
	case reuseaddr:
		lc.Control = reuseaddrControl
	}

	return lc.Listen(context.Background(), network, addr)
}

func listenUDP(network, addr string, reuseport, reuseaddr bool) (net.PacketConn, error) {
	var lc net.ListenConfig
	switch {
	case reuseaddr && reuseport:
	case reuseport:
		lc.Control = reuseportControl
	case reuseaddr:
		lc.Control = reuseaddrControl
	}

	return lc.ListenPacket(context.Background(), network, addr)
}
