// Package sockets provides helper functions to create and configure Unix or TCP sockets.
package sockets

import (
	"errors"
	"net"
	"net/http"
	"time"
)

const defaultTimeout = 10 * time.Second

// ErrProtocolNotAvailable is returned when a given transport protocol is not provided by the operating system.
var ErrProtocolNotAvailable = errors.New("protocol not available")

// ConfigureTransport configures the specified [http.Transport] according to the specified proto
// and addr.
//
// If the proto is unix (using a unix socket to communicate) or npipe the compression is disabled.
// For other protos, compression is enabled. If you want to manually enable/disable compression,
// make sure you do it _after_ any subsequent calls to ConfigureTransport is made against the same
// [http.Transport].
func ConfigureTransport(tr *http.Transport, proto, addr string) error {
	switch proto {
	case "unix":
		return configureUnixTransport(tr, proto, addr)
	case "npipe":
		return configureNpipeTransport(tr, proto, addr)
	default:
		tr.Proxy = http.ProxyFromEnvironment
		tr.DisableCompression = false
		tr.DialContext = (&net.Dialer{
			Timeout: defaultTimeout,
		}).DialContext
	}
	return nil
}
