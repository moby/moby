//go:build !aix && !darwin && !dragonfly && !freebsd && !linux && !netbsd && !openbsd
// +build !aix,!darwin,!dragonfly,!freebsd,!linux,!netbsd,!openbsd

package dns

import "net"

const supportsReusePort = false

func listenTCP(network, addr string, reuseport, reuseaddr bool) (net.Listener, error) {
	if reuseport || reuseaddr {
		// TODO(tmthrgd): return an error?
	}

	return net.Listen(network, addr)
}

const supportsReuseAddr = false

func listenUDP(network, addr string, reuseport, reuseaddr bool) (net.PacketConn, error) {
	if reuseport || reuseaddr {
		// TODO(tmthrgd): return an error?
	}

	return net.ListenPacket(network, addr)
}
