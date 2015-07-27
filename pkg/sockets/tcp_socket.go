// Package sockets provides helper functions to create and configure Unix or TCP
// sockets.
package sockets

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/docker/docker/pkg/listenbuffer"
)

// NewTCPSocket creates a TCP socket listener with the specified address and
// and the specified tls configuration. If TLSConfig is set, will encapsulate the
// TCP listener inside a TLS one.
// The channel passed is used to activate the listenbuffer when the caller is ready
// to accept connections.
func NewTCPSocket(addr string, tlsConfig *tls.Config, activate <-chan struct{}) (net.Listener, error) {
	l, err := listenbuffer.NewListenBuffer("tcp", addr, activate)
	if err != nil {
		return nil, err
	}
	if tlsConfig != nil {
		tlsConfig.NextProtos = []string{"http/1.1"}
		l = tls.NewListener(l, tlsConfig)
	}
	return l, nil
}

// ConfigureTCPTransport configures the specified Transport according to the
// specified proto and addr.
// If the proto is unix (using a unix socket to communicate) the compression
// is disabled.
func ConfigureTCPTransport(tr *http.Transport, proto, addr string) {
	// Why 32? See https://github.com/docker/docker/pull/8035.
	timeout := 32 * time.Second
	if proto == "unix" {
		// No need for compression in local communications.
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, timeout)
		}
	} else {
		tr.Proxy = http.ProxyFromEnvironment
		tr.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}
}
