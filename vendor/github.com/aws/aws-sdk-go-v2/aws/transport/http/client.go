package http

import (
	"context"
	"crypto/fips140"
	"crypto/tls"
	"net"
	"net/http"
	"reflect"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/smithy-go/tracing"
)

// Defaults for the HTTPTransportBuilder.
var (
	// Default connection pool options
	DefaultHTTPTransportMaxIdleConns        = 100
	DefaultHTTPTransportMaxIdleConnsPerHost = 10
	DefaultHTTPTransportMaxConnsPerHost     = 2048

	// Default connection timeouts
	DefaultHTTPTransportIdleConnTimeout       = 90 * time.Second
	DefaultHTTPTransportTLSHandleshakeTimeout = 10 * time.Second
	DefaultHTTPTransportExpectContinueTimeout = 1 * time.Second

	// Default to TLS 1.2 for all HTTPS requests.
	DefaultHTTPTransportTLSMinVersion uint16 = tls.VersionTLS12

	// DefaultHTTPTransportTLSCurvePreferencesFIPS is the elliptic curve preference
	// list applied to the default transport when the FIPS 140-3 module is active.
	//
	// Go's default preferences lead with X25519, which crypto/ecdh rejects under
	// GODEBUG=fips140=only, failing every TLS handshake the SDK attempts. Only the
	// NIST curves are FIPS-approved, so restricting to them keeps the default
	// client usable in FIPS deployments.
	DefaultHTTPTransportTLSCurvePreferencesFIPS = []tls.CurveID{
		tls.CurveP256,
		tls.CurveP384,
		tls.CurveP521,
	}
)

// Timeouts for net.Dialer's network connection.
var (
	DefaultDialConnectTimeout   = 30 * time.Second
	DefaultDialKeepAliveTimeout = 30 * time.Second
)

// BuildableClient provides a HTTPClient implementation with options to
// create copies of the HTTPClient when additional configuration is provided.
//
// The client's methods will not share the http.Transport value between copies
// of the BuildableClient. Only exported member values of the Transport and
// optional Dialer will be copied between copies of BuildableClient.
type BuildableClient struct {
	transport *http.Transport
	dialer    *net.Dialer

	initOnce sync.Once

	clientTimeout time.Duration
	readTimeout   *time.Duration
	client        *http.Client
}

// NewBuildableClient returns an initialized client for invoking HTTP
// requests.
func NewBuildableClient() *BuildableClient {
	return &BuildableClient{}
}

// Do implements the HTTPClient interface's Do method to invoke a HTTP request,
// and receive the response. Uses the BuildableClient's current
// configuration to invoke the http.Request.
//
// If connection pooling is enabled (aka HTTP KeepAlive) the client will only
// share pooled connections with its own instance. Copies of the
// BuildableClient will have their own connection pools.
//
// Redirect (3xx) responses will not be followed, the HTTP response received
// will returned instead.
func (b *BuildableClient) Do(req *http.Request) (*http.Response, error) {
	b.initOnce.Do(b.build)

	return b.client.Do(req)
}

// Freeze returns a frozen aws.HTTPClient implementation that is no longer a BuildableClient.
// Use this to prevent the SDK from applying DefaultMode configuration values to a buildable client.
func (b *BuildableClient) Freeze() aws.HTTPClient {
	cpy := b.clone()
	cpy.build()
	return cpy.client
}

func (b *BuildableClient) build() {
	tr := b.GetTransport()
	b.installReadTimeout(tr)

	b.client = wrapWithLimitedRedirect(&http.Client{
		Timeout:   b.clientTimeout,
		Transport: tr,
	})
}

func (b *BuildableClient) clone() *BuildableClient {
	cpy := NewBuildableClient()
	cpy.transport = b.GetTransport()
	cpy.dialer = b.GetDialer()
	cpy.clientTimeout = b.clientTimeout
	cpy.readTimeout = b.readTimeout

	return cpy
}

// WithTransportOptions copies the BuildableClient and returns it with the
// http.Transport options applied.
//
// If a non (*http.Transport) was set as the round tripper, the round tripper
// will be replaced with a default Transport value before invoking the option
// functions.
func (b *BuildableClient) WithTransportOptions(opts ...func(*http.Transport)) *BuildableClient {
	cpy := b.clone()

	tr := cpy.GetTransport()
	for _, opt := range opts {
		opt(tr)
	}
	cpy.transport = tr

	return cpy
}

// WithDialerOptions copies the BuildableClient and returns it with the
// net.Dialer options applied. Will set the client's http.Transport DialContext
// member.
func (b *BuildableClient) WithDialerOptions(opts ...func(*net.Dialer)) *BuildableClient {
	cpy := b.clone()

	dialer := cpy.GetDialer()
	for _, opt := range opts {
		opt(dialer)
	}
	cpy.dialer = dialer

	tr := cpy.GetTransport()
	tr.DialContext = cpy.dialer.DialContext
	cpy.transport = tr

	return cpy
}

// WithTimeout Sets the timeout used by the client for all requests.
func (b *BuildableClient) WithTimeout(timeout time.Duration) *BuildableClient {
	cpy := b.clone()
	cpy.clientTimeout = timeout
	return cpy
}

// WithReadTimeout copies the BuildableClient and returns it with the read
// timeout set.
//
// The timeout is the maximum time the client waits for a connection to deliver
// any data. It resets on every byte received, so a slow but progressing response
// does not fail. It is not a deadline on the operation; use WithTimeout for that.
//
// A value set here takes precedence over the SDK's defaults for every service
// this client is used with, including services the SDK would otherwise apply a
// higher value to or exempt entirely. Pass 0 to disable read timeouts.
//
// The timeout is applied per connection, so a client shared between service
// clients applies the same value to all of them.
func (b *BuildableClient) WithReadTimeout(timeout time.Duration) *BuildableClient {
	cpy := b.clone()
	cpy.readTimeout = &timeout
	return cpy
}

// GetTransport returns a copy of the client's HTTP Transport.
func (b *BuildableClient) GetTransport() *http.Transport {
	var tr *http.Transport
	if b.transport != nil {
		tr = b.transport.Clone()
	} else {
		tr = defaultHTTPTransport()
	}

	return tr
}

// GetDialer returns a copy of the client's network dialer.
func (b *BuildableClient) GetDialer() *net.Dialer {
	var dialer *net.Dialer
	if b.dialer != nil {
		dialer = shallowCopyStruct(b.dialer).(*net.Dialer)
	} else {
		dialer = defaultDialer()
	}

	return dialer
}

// GetTimeout returns a copy of the client's timeout to cancel requests with.
func (b *BuildableClient) GetTimeout() time.Duration {
	return b.clientTimeout
}

// GetReadTimeout returns the configured read timeout and whether one was set on
// this client. When it was not, the SDK resolves a default per
// service.
func (b *BuildableClient) GetReadTimeout() (time.Duration, bool) {
	if b.readTimeout == nil {
		return 0, false
	}

	return *b.readTimeout, true
}

func defaultDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   DefaultDialConnectTimeout,
		KeepAlive: DefaultDialKeepAliveTimeout,
		DualStack: true,
	}
}

// defaultTLSCurvePreferences returns the curve preferences for the default
// transport. Outside FIPS mode it returns nil so Go's own defaults apply,
// preserving X25519 and the post-quantum X25519MLKEM768 hybrid.
func defaultTLSCurvePreferences(fipsEnabled bool) []tls.CurveID {
	if !fipsEnabled {
		return nil
	}
	return DefaultHTTPTransportTLSCurvePreferencesFIPS
}

func defaultHTTPTransport() *http.Transport {
	dialer := defaultDialer()

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           traceDialContext(dialer.DialContext),
		TLSHandshakeTimeout:   DefaultHTTPTransportTLSHandleshakeTimeout,
		MaxIdleConns:          DefaultHTTPTransportMaxIdleConns,
		MaxIdleConnsPerHost:   DefaultHTTPTransportMaxIdleConnsPerHost,
		MaxConnsPerHost:       DefaultHTTPTransportMaxConnsPerHost,
		IdleConnTimeout:       DefaultHTTPTransportIdleConnTimeout,
		ExpectContinueTimeout: DefaultHTTPTransportExpectContinueTimeout,
		ForceAttemptHTTP2:     true,
		TLSClientConfig: &tls.Config{
			MinVersion:       DefaultHTTPTransportTLSMinVersion,
			CurvePreferences: defaultTLSCurvePreferences(fips140.Enabled()),
		},
	}

	return tr
}

type dialContext func(ctx context.Context, network, addr string) (net.Conn, error)

func traceDialContext(dc dialContext) dialContext {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		span, _ := tracing.GetSpan(ctx)
		span.SetProperty("net.peer.name", addr)

		conn, err := dc(ctx, network, addr)
		if err != nil {
			return conn, err
		}

		raddr := conn.RemoteAddr()
		if raddr == nil {
			return conn, err
		}

		host, port, err := net.SplitHostPort(raddr.String())
		if err != nil { // don't blow up just because we couldn't parse
			span.SetProperty("net.peer.addr", raddr.String())
		} else {
			span.SetProperty("net.peer.host", host)
			span.SetProperty("net.peer.port", port)
		}

		return conn, err
	}
}

// shallowCopyStruct creates a shallow copy of the passed in source struct, and
// returns that copy of the same struct type.
func shallowCopyStruct(src interface{}) interface{} {
	srcVal := reflect.ValueOf(src)
	srcValType := srcVal.Type()

	var returnAsPtr bool
	if srcValType.Kind() == reflect.Ptr {
		srcVal = srcVal.Elem()
		srcValType = srcValType.Elem()
		returnAsPtr = true
	}
	dstVal := reflect.New(srcValType).Elem()

	for i := 0; i < srcValType.NumField(); i++ {
		ft := srcValType.Field(i)
		if len(ft.PkgPath) != 0 {
			// unexported fields have a PkgPath
			continue
		}

		dstVal.Field(i).Set(srcVal.Field(i))
	}

	if returnAsPtr {
		dstVal = dstVal.Addr()
	}

	return dstVal.Interface()
}

// wrapWithLimitedRedirect updates the Client's Transport and CheckRedirect to
// not follow any redirect other than 307 and 308. No other redirect will be
// followed.
//
// If the client does not have a Transport defined will use a new SDK default
// http.Transport configuration.
func wrapWithLimitedRedirect(c *http.Client) *http.Client {
	tr := c.Transport
	if tr == nil {
		tr = defaultHTTPTransport()
	}

	cc := *c
	cc.CheckRedirect = limitedRedirect
	cc.Transport = suppressBadHTTPRedirectTransport{
		tr: tr,
	}

	return &cc
}

// limitedRedirect is a CheckRedirect that prevents the client from following
// any non 307/308 HTTP status code redirects.
//
// The 307 and 308 redirects are allowed because the client must use the
// original HTTP method for the redirected to location. Whereas 301 and 302
// allow the client to switch to GET for the redirect.
//
// Suppresses all redirect requests with a URL of badHTTPRedirectLocation.
func limitedRedirect(r *http.Request, via []*http.Request) error {
	// Request.Response, in CheckRedirect is the response that is triggering
	// the redirect.
	resp := r.Response
	if r.URL.String() == badHTTPRedirectLocation {
		resp.Header.Del(badHTTPRedirectLocation)
		return http.ErrUseLastResponse
	}

	switch resp.StatusCode {
	case 307, 308:
		// Only allow 307 and 308 redirects as they preserve the method.

		// If redirecting to a different host, remove X-Amz-Security-Token header
		// to prevent credentials from being sent to a different host, similar to
		// how Authorization header is handled by the HTTP client.
		if len(via) > 0 {
			lastRequest := via[len(via)-1]
			if lastRequest.URL.Host != r.URL.Host {
				r.Header.Del("X-Amz-Security-Token")
			}
		}

		return nil
	}

	return http.ErrUseLastResponse
}

// suppressBadHTTPRedirectTransport provides an http.RoundTripper
// implementation that wraps another http.RoundTripper to prevent HTTP client
// receiving 301 and 302 HTTP responses redirects without the required location
// header.
//
// Clients using this utility must have a CheckRedirect, e.g. limitedRedirect,
// that check for responses with having a URL of baseHTTPRedirectLocation, and
// suppress the redirect.
type suppressBadHTTPRedirectTransport struct {
	tr http.RoundTripper
}

const badHTTPRedirectLocation = `https://amazonaws.com/badhttpredirectlocation`

// RoundTrip backfills a stub location when a 301/302 response is received
// without a location. This stub location is used by limitedRedirect to prevent
// the HTTP client from failing attempting to use follow a redirect without a
// location value.
func (t suppressBadHTTPRedirectTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	resp, err := t.tr.RoundTrip(r)
	if err != nil {
		return resp, err
	}

	// S3 is the only known service to return 301 without location header.
	// The Go standard library HTTP client will return an opaque error if it
	// tries to follow a 301/302 response missing the location header.
	switch resp.StatusCode {
	case 301, 302:
		if v := resp.Header.Get("Location"); len(v) == 0 {
			resp.Header.Set("Location", badHTTPRedirectLocation)
		}
	}

	return resp, err
}
