// Package sockets provides helper functions to create and configure Unix or TCP sockets.
package sockets

import (
	"crypto/tls"
	"net"
)

// NewTCPSocket creates a TCP socket listener with the specified address and
// the specified tls configuration. If TLSConfig is set, will encapsulate the
// TCP listener inside a TLS one.
// If no ALPN protocols are configured, it defaults to HTTP/1.1.
func NewTCPSocket(addr string, tlsConfig *tls.Config) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		if len(tlsConfig.NextProtos) == 0 {
			tlsConfig = tlsConfig.Clone()
			tlsConfig.NextProtos = []string{"http/1.1"}
		}
		l = tls.NewListener(l, tlsConfig)
	}
	return l, nil
}
