// docker-proxy provides a network Proxy interface and implementations for TCP
// and UDP.
package main

import (
	"net"

	"github.com/ishidawataru/sctp"
)

// ipVersion refers to IP version - v4 or v6
type ipVersion string

const (
	// IPv4 is version 4
	ipv4 ipVersion = "4"
	// IPv4 is version 6
	ipv6 ipVersion = "6"
)

// Proxy defines the behavior of a proxy. It forwards traffic back and forth
// between two endpoints : the frontend and the backend.
// It can be used to do software port-mapping between two addresses.
// e.g. forward all traffic between the frontend (host) 127.0.0.1:3000
// to the backend (container) at 172.17.42.108:4000.
type Proxy interface {
	// Run starts forwarding traffic back and forth between the front
	// and back-end addresses.
	Run()
	// Close stops forwarding traffic and close both ends of the Proxy.
	Close()
	// FrontendAddr returns the address on which the proxy is listening.
	FrontendAddr() net.Addr
	// BackendAddr returns the proxied address.
	BackendAddr() net.Addr
}

// NewProxy creates a Proxy according to the specified frontendAddr and backendAddr.
func NewProxy(frontendAddr, backendAddr net.Addr) (Proxy, error) {
	switch frontendAddr.(type) {
	case *net.UDPAddr:
		return NewUDPProxy(frontendAddr.(*net.UDPAddr), backendAddr.(*net.UDPAddr))
	case *net.TCPAddr:
		return NewTCPProxy(frontendAddr.(*net.TCPAddr), backendAddr.(*net.TCPAddr))
	case *sctp.SCTPAddr:
		return NewSCTPProxy(frontendAddr.(*sctp.SCTPAddr), backendAddr.(*sctp.SCTPAddr))
	default:
		panic("Unsupported protocol")
	}
}
