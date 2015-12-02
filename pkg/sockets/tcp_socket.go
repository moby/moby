// Package sockets provides helper functions to create and configure Unix or TCP
// sockets.
package sockets

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/proxy"
)

// GetProxyEnv allows access to the uppercase and the lowercase forms of
// proxy-related variables.  See the Go specification for details on these
// variables. https://golang.org/pkg/net/http/
func GetProxyEnv(key string) string {
	proxyValue := os.Getenv(strings.ToUpper(key))
	if proxyValue == "" {
		return os.Getenv(strings.ToLower(key))
	}
	return proxyValue
}

// DialerFromEnvironment takes in a "direct" *net.Dialer and returns a
// proxy.Dialer which will route the connections through the proxy using the
// given dialer.
func DialerFromEnvironment(direct *net.Dialer) (proxy.Dialer, error) {
	allProxy := GetProxyEnv("all_proxy")
	if len(allProxy) == 0 {
		return direct, nil
	}

	proxyURL, err := url.Parse(allProxy)
	if err != nil {
		return direct, err
	}

	proxyFromURL, err := proxy.FromURL(proxyURL, direct)
	if err != nil {
		return direct, err
	}

	noProxy := GetProxyEnv("no_proxy")
	if len(noProxy) == 0 {
		return proxyFromURL, nil
	}

	perHost := proxy.NewPerHost(proxyFromURL, direct)
	perHost.AddFromString(noProxy)

	return perHost, nil
}

// NewTCPSocket creates a TCP socket listener with the specified address and
// and the specified tls configuration. If TLSConfig is set, will encapsulate the
// TCP listener inside a TLS one.
func NewTCPSocket(addr string, tlsConfig *tls.Config) (net.Listener, error) {
	l, err := net.Listen("tcp", addr)
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
func ConfigureTCPTransport(tr *http.Transport, proto, addr string) error {
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
		dialer, err := DialerFromEnvironment(&net.Dialer{
			Timeout: timeout,
		})

		if err != nil {
			if strings.Contains(err.Error(), "unknown scheme") {
				return fmt.Errorf("%s. Did you mean 'socks5://'?", err)
			}
			return err
		}
		tr.Dial = dialer.Dial
	}

	return nil
}
