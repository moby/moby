// Package sockets provides helper functions to create and configure Unix or TCP sockets.
package sockets

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"syscall"
	"time"
)

const (
	defaultTimeout        = 10 * time.Second
	maxUnixSocketPathSize = len(syscall.RawSockaddrUnix{}.Path)
)

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
	if tr.MaxIdleConns == 0 {
		// prevent long-lived processes from leaking connections
		// due to idle connections not being released.
		//
		// TODO: see if we can also address this from the server side; see: https://github.com/moby/moby/issues/45539
		tr.MaxIdleConns = 6
		tr.IdleConnTimeout = 30 * time.Second
	}
	switch proto {
	case "unix":
		return configureUnixTransport(tr, addr)
	case "npipe":
		return configureNpipeTransport(tr, addr)
	default:
		tr.Proxy = http.ProxyFromEnvironment
		tr.DisableCompression = false
		tr.DialContext = (&net.Dialer{
			Timeout: defaultTimeout,
		}).DialContext
	}
	return nil
}

func configureUnixTransport(tr *http.Transport, addr string) error {
	if len(addr) > maxUnixSocketPathSize {
		return fmt.Errorf("unix socket path %q is too long", addr)
	}
	// No need for compression in local communications.
	tr.DisableCompression = true
	dialer := &net.Dialer{
		Timeout: defaultTimeout,
	}
	tr.DialContext = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "unix", addr)
	}
	return nil
}
