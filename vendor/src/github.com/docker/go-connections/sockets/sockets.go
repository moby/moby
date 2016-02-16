// Package sockets provides helper functions to create and configure Unix or TCP sockets.
package sockets

import (
	"net"
	"net/http"
	"time"
)

// Why 32? See https://github.com/docker/docker/pull/8035.
const defaulTimeout = 32 * time.Second

// ConfigureTransport configures the specified Transport according to the
// specified proto and addr.
// If the proto is unix (using a unix socket to communicate) the compression
// is disabled.
func ConfigureTransport(tr *http.Transport, proto, addr string) {
	switch proto {
	case "unix":
		// No need for compression in local communications.
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return net.DialTimeout(proto, addr, defaulTimeout)
		}
	case "npipe":
		// No need for compression in local communications.
		tr.DisableCompression = true
		tr.Dial = func(_, _ string) (net.Conn, error) {
			return DialPipe(addr, defaulTimeout)
		}
	default:
		tr.Proxy = http.ProxyFromEnvironment
		tr.Dial = (&net.Dialer{Timeout: defaulTimeout}).Dial
	}
}
