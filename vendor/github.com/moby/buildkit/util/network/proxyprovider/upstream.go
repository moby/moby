package proxyprovider

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/pkg/errors"
	"golang.org/x/net/idna"
)

// upstreamProxyEnvironment validates the proxy environment before a proxy
// namespace starts. The boolean reports whether an HTTPS proxy needs the
// transport compatibility wrapper below.
func upstreamProxyEnvironment() (bool, error) {
	var hasHTTPSProxy bool
	for _, names := range [][2]string{
		{"HTTP_PROXY", "http_proxy"},
		{"HTTPS_PROXY", "https_proxy"},
	} {
		name, value := proxyEnvironmentValue(names[0], names[1])
		if value == "" {
			continue
		}
		proxyURL, err := parseProxyEnvironmentValue(value)
		if err != nil {
			// Proxy URLs may contain credentials, so neither the value nor the
			// parser error is safe to include here.
			return false, errors.Errorf("invalid %s in buildkitd environment", name)
		}
		if proxyURL.Scheme == "https" {
			hasHTTPSProxy = true
		}
	}
	return hasHTTPSProxy, nil
}

func proxyEnvironmentValue(names ...string) (string, string) {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return name, value
		}
	}
	return "", ""
}

// parseProxyEnvironmentValue matches the URL handling used by
// http.ProxyFromEnvironment, including treating host[:port] as an HTTP proxy.
func parseProxyEnvironmentValue(value string) (*url.URL, error) {
	proxyURL, err := url.Parse(value)
	if err != nil || proxyURL.Scheme == "" || proxyURL.Host == "" {
		proxyURL, err = url.Parse("http://" + value)
	}
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if proxyURL.Hostname() == "" {
		return nil, errors.Errorf("proxy URL with scheme %q is missing host", proxyURL.Scheme)
	}
	switch proxyURL.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, errors.Errorf("unsupported proxy URL scheme %q", proxyURL.Scheme)
	}
	return proxyURL, nil
}

func canonicalHTTPSProxyAddr(proxyURL *url.URL) string {
	host := proxyURL.Hostname()
	if strings.IndexFunc(host, func(r rune) bool { return r >= utf8.RuneSelf }) >= 0 {
		if asciiHost, err := idna.Lookup.ToASCII(host); err == nil {
			host = asciiHost
		}
	}
	port := proxyURL.Port()
	if port == "" {
		port = "443"
	}
	return net.JoinHostPort(host, port)
}

type upstreamProxySelectionKey struct{}

type upstreamProxySelection struct {
	once      sync.Once
	proxyURL  *url.URL
	err       error
	httpsAddr string
}

func (s *upstreamProxySelection) proxyForRequest(req *http.Request, proxyFunc func(*http.Request) (*url.URL, error)) (*url.URL, error) {
	// Transport may retry a request while a dial from an earlier attempt is
	// still running. Pin the decision so concurrent dials see immutable state
	// and retries use the same proxy.
	s.once.Do(func() {
		s.proxyURL, s.err = proxyFunc(req)
		if s.err == nil && s.proxyURL != nil && s.proxyURL.Scheme == "https" {
			s.httpsAddr = canonicalHTTPSProxyAddr(s.proxyURL)
		}
	})
	return s.proxyURL, s.err
}

type upstreamProxyRoundTripper struct {
	transport *http.Transport
}

func (t *upstreamProxyRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	selection := &upstreamProxySelection{}
	ctx := context.WithValue(req.Context(), upstreamProxySelectionKey{}, selection)
	return t.transport.RoundTrip(req.WithContext(ctx))
}

// configureTransportForUpstream disables HTTP/2 negotiation with an HTTPS
// proxy. Direct and tunneled origin connections retain HTTP/2 support. The
// standard transport sends CONNECT to forward proxies using HTTP/1.1 and does
// not support HTTP/2 proxy connections.
func configureTransportForUpstream(transport *http.Transport) http.RoundTripper {
	proxyFunc := transport.Proxy
	if proxyFunc == nil {
		return transport
	}
	transport.Proxy = func(req *http.Request) (*url.URL, error) {
		selection, _ := req.Context().Value(upstreamProxySelectionKey{}).(*upstreamProxySelection)
		if selection == nil {
			return proxyFunc(req)
		}
		return selection.proxyForRequest(req, proxyFunc)
	}
	dialContext := transport.DialContext
	if dialContext == nil {
		dialer := &net.Dialer{}
		dialContext = dialer.DialContext
	}
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, addr)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		tlsConfig := transport.TLSClientConfig
		if tlsConfig == nil {
			tlsConfig = &tls.Config{}
		} else {
			tlsConfig = tlsConfig.Clone()
		}
		if tlsConfig.ServerName == "" {
			tlsConfig.ServerName, _, err = net.SplitHostPort(addr)
			if err != nil {
				_ = conn.Close()
				return nil, errors.WithStack(err)
			}
		}
		selection, _ := ctx.Value(upstreamProxySelectionKey{}).(*upstreamProxySelection)
		if selection != nil && selection.httpsAddr == addr {
			tlsConfig.NextProtos = []string{"http/1.1"}
		}
		tlsConn := tls.Client(conn, tlsConfig)
		handshakeCtx := ctx
		if transport.TLSHandshakeTimeout != 0 {
			var cancel context.CancelFunc
			handshakeCtx, cancel = context.WithTimeoutCause(ctx, transport.TLSHandshakeTimeout, errors.WithStack(context.DeadlineExceeded))
			defer cancel()
		}
		if err := tlsConn.HandshakeContext(handshakeCtx); err != nil {
			_ = conn.Close()
			return nil, errors.WithStack(err)
		}
		return tlsConn, nil
	}
	return &upstreamProxyRoundTripper{transport: transport}
}
