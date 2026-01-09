package client

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"
)

type clientConfig struct {
	// scheme sets the scheme for the client
	scheme string
	// host holds the server address to connect to
	host string
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
	// basePath holds the path to prepend to the requests.
	basePath string
	// client used to send and receive http requests.
	client *http.Client
	// version of the server to talk to.
	version string
	// userAgent is the User-Agent header to use for HTTP requests. It takes
	// precedence over User-Agent headers set in customHTTPHeaders, and other
	// header variables. When set to an empty string, the User-Agent header
	// is removed, and no header is sent.
	userAgent *string
	// custom HTTP headers configured by users.
	customHTTPHeaders map[string]string

	// manualAPIVersion contains the API version set by users. This field
	// will only be non-empty if a valid-formed version was set through
	// [WithAPIVersion].
	//
	// If both manualAPIVersion and envAPIVersion are set, manualAPIVersion
	// takes precedence. Either field disables API-version negotiation.
	manualAPIVersion string

	// envAPIVersion contains the API version set by users. This field
	// will only be non-empty if a valid-formed version was set through
	// [WithAPIVersionFromEnv].
	//
	// If both manualAPIVersion and envAPIVersion are set, manualAPIVersion
	// takes precedence. Either field disables API-version negotiation.
	envAPIVersion string

	// responseHooks is a list of custom response hooks to call on responses.
	responseHooks []ResponseHook

	// traceOpts is a list of options to configure the tracing span.
	traceOpts []otelhttp.Option
}

// ResponseHook is called for each HTTP response returned by the daemon.
// Hooks are invoked in the order they were added.
//
// Hooks must not read or close resp.Body.
type ResponseHook func(*http.Response) error

// Opt is a configuration option to initialize a [Client].
type Opt func(*clientConfig) error

// FromEnv configures the client with values from environment variables. It
// is the equivalent of using the [WithTLSClientConfigFromEnv], [WithHostFromEnv],
// and [WithAPIVersionFromEnv] options.
//
// FromEnv uses the following environment variables:
//
//   - DOCKER_HOST ([EnvOverrideHost]) to set the URL to the docker server.
//   - DOCKER_API_VERSION ([EnvOverrideAPIVersion]) to set the version of the
//     API to use, leave empty for latest.
//   - DOCKER_CERT_PATH ([EnvOverrideCertPath]) to specify the directory from
//     which to load the TLS certificates ("ca.pem", "cert.pem", "key.pem').
//   - DOCKER_TLS_VERIFY ([EnvTLSVerify]) to enable or disable TLS verification
//     (off by default).
func FromEnv(c *clientConfig) error {
	ops := []Opt{
		WithTLSClientConfigFromEnv(),
		WithHostFromEnv(),
		WithAPIVersionFromEnv(),
	}
	for _, op := range ops {
		if err := op(c); err != nil {
			return err
		}
	}
	return nil
}

// WithDialContext applies the dialer to the client transport. This can be
// used to set the Timeout and KeepAlive settings of the client. It returns
// an error if the client does not have a [http.Transport] configured.
func WithDialContext(dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) Opt {
	return func(c *clientConfig) error {
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			transport.DialContext = dialContext
			return nil
		}
		return fmt.Errorf("cannot apply dialer to transport: %T", c.client.Transport)
	}
}

// WithHost overrides the client host with the specified one.
func WithHost(host string) Opt {
	return func(c *clientConfig) error {
		hostURL, err := ParseHostURL(host)
		if err != nil {
			return err
		}
		c.host = host
		c.proto = hostURL.Scheme
		c.addr = hostURL.Host
		c.basePath = hostURL.Path
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			return sockets.ConfigureTransport(transport, c.proto, c.addr)
		}
		// For test transports, we skip transport configuration but still
		// set the host fields so that the client can use them for headers
		if _, ok := c.client.Transport.(testRoundTripper); ok {
			return nil
		}
		return fmt.Errorf("cannot apply host to transport: %T", c.client.Transport)
	}
}

// testRoundTripper allows us to inject a mock-transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type testRoundTripper func(*http.Request) (*http.Response, error)

func (tf testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

// WithHostFromEnv overrides the client host with the host specified in the
// DOCKER_HOST ([EnvOverrideHost]) environment variable. If DOCKER_HOST is not set,
// or set to an empty value, the host is not modified.
func WithHostFromEnv() Opt {
	return func(c *clientConfig) error {
		if host := os.Getenv(EnvOverrideHost); host != "" {
			return WithHost(host)(c)
		}
		return nil
	}
}

// WithHTTPClient overrides the client's HTTP client with the specified one.
func WithHTTPClient(client *http.Client) Opt {
	return func(c *clientConfig) error {
		if client != nil {
			// Make a clone of client so modifications do not affect
			// the caller's client. Clone here instead of in New()
			// as other options (WithHost) also mutate c.client.
			// Cloned clients share the same CookieJar as the
			// original.
			hc := *client
			if ht, ok := hc.Transport.(*http.Transport); ok {
				hc.Transport = ht.Clone()
			}
			c.client = &hc
		}
		return nil
	}
}

// WithTimeout configures the time limit for requests made by the HTTP client.
func WithTimeout(timeout time.Duration) Opt {
	return func(c *clientConfig) error {
		c.client.Timeout = timeout
		return nil
	}
}

// WithUserAgent configures the User-Agent header to use for HTTP requests.
// It overrides any User-Agent set in headers. When set to an empty string,
// the User-Agent header is removed, and no header is sent.
func WithUserAgent(ua string) Opt {
	return func(c *clientConfig) error {
		c.userAgent = &ua
		return nil
	}
}

// WithHTTPHeaders appends custom HTTP headers to the client's default headers.
// It does not allow for built-in headers (such as "User-Agent", if set) to
// be overridden. Also see [WithUserAgent].
func WithHTTPHeaders(headers map[string]string) Opt {
	return func(c *clientConfig) error {
		c.customHTTPHeaders = headers
		return nil
	}
}

// WithScheme overrides the client scheme with the specified one.
func WithScheme(scheme string) Opt {
	return func(c *clientConfig) error {
		c.scheme = scheme
		return nil
	}
}

// WithTLSClientConfig applies a TLS config to the client transport.
func WithTLSClientConfig(cacertPath, certPath, keyPath string) Opt {
	return func(c *clientConfig) error {
		transport, ok := c.client.Transport.(*http.Transport)
		if !ok {
			return fmt.Errorf("cannot apply tls config to transport: %T", c.client.Transport)
		}
		config, err := tlsconfig.Client(tlsconfig.Options{
			CAFile:             cacertPath,
			CertFile:           certPath,
			KeyFile:            keyPath,
			ExclusiveRootPools: true,
		})
		if err != nil {
			return fmt.Errorf("failed to create tls config: %w", err)
		}
		transport.TLSClientConfig = config
		return nil
	}
}

// WithTLSClientConfigFromEnv configures the client's TLS settings with the
// settings in the DOCKER_CERT_PATH ([EnvOverrideCertPath]) and DOCKER_TLS_VERIFY
// ([EnvTLSVerify]) environment variables. If DOCKER_CERT_PATH is not set or empty,
// TLS configuration is not modified.
//
// WithTLSClientConfigFromEnv uses the following environment variables:
//
//   - DOCKER_CERT_PATH ([EnvOverrideCertPath]) to specify the directory from
//     which to load the TLS certificates ("ca.pem", "cert.pem", "key.pem").
//   - DOCKER_TLS_VERIFY ([EnvTLSVerify]) to enable or disable TLS verification
//     (off by default).
func WithTLSClientConfigFromEnv() Opt {
	return func(c *clientConfig) error {
		dockerCertPath := os.Getenv(EnvOverrideCertPath)
		if dockerCertPath == "" {
			return nil
		}
		tlsc, err := tlsconfig.Client(tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv(EnvTLSVerify) == "",
		})
		if err != nil {
			return err
		}

		c.client = &http.Client{
			Transport:     &http.Transport{TLSClientConfig: tlsc},
			CheckRedirect: CheckRedirect,
		}
		return nil
	}
}

// WithAPIVersion overrides the client's API version with the specified one,
// and disables API version negotiation. If an empty version is provided,
// this option is ignored to allow version negotiation. The given version
// should be formatted "<major>.<minor>" (for example, "1.52"). It returns
// an error if the given value not in the correct format.
//
// WithAPIVersion does not validate if the client supports the given version,
// and callers should verify if the version lower than the maximum supported
// version as defined by [MaxAPIVersion].
//
// [WithAPIVersionFromEnv] takes precedence if [WithAPIVersion] and
// [WithAPIVersionFromEnv] are both set.
func WithAPIVersion(version string) Opt {
	return func(c *clientConfig) error {
		version = strings.TrimSpace(version)
		if val := strings.TrimPrefix(version, "v"); val != "" {
			ver, err := parseAPIVersion(val)
			if err != nil {
				return fmt.Errorf("invalid API version (%s): %w", version, err)
			}
			c.manualAPIVersion = ver
		}
		return nil
	}
}

// WithVersion overrides the client version with the specified one.
//
// Deprecated: use [WithAPIVersion] instead.
func WithVersion(version string) Opt {
	return WithAPIVersion(version)
}

// WithAPIVersionFromEnv overrides the client version with the version specified in
// the DOCKER_API_VERSION ([EnvOverrideAPIVersion]) environment variable.
// If DOCKER_API_VERSION is not set, or set to an empty value, the version
// is not modified.
//
// WithAPIVersion does not validate if the client supports the given version,
// and callers should verify if the version lower than the maximum supported
// version as defined by [MaxAPIVersion].
//
// [WithAPIVersionFromEnv] takes precedence if [WithAPIVersion] and
// [WithAPIVersionFromEnv] are both set.
func WithAPIVersionFromEnv() Opt {
	return func(c *clientConfig) error {
		version := strings.TrimSpace(os.Getenv(EnvOverrideAPIVersion))
		if val := strings.TrimPrefix(version, "v"); val != "" {
			ver, err := parseAPIVersion(val)
			if err != nil {
				return fmt.Errorf("invalid API version (%s): %w", version, err)
			}
			c.envAPIVersion = ver
		}
		return nil
	}
}

// WithVersionFromEnv overrides the client version with the version specified in
// the DOCKER_API_VERSION ([EnvOverrideAPIVersion]) environment variable.
//
// Deprecated: use [WithAPIVersionFromEnv] instead.
func WithVersionFromEnv() Opt {
	return WithAPIVersionFromEnv()
}

// WithAPIVersionNegotiation enables automatic API version negotiation for the client.
// With this option enabled, the client automatically negotiates the API version
// to use when making requests. API version negotiation is performed on the first
// request; subsequent requests do not re-negotiate.
//
// Deprecated: API-version negotiation is now enabled by default. Use [WithAPIVersion]
// or [WithAPIVersionFromEnv] to disable API version negotiation.
func WithAPIVersionNegotiation() Opt {
	return func(c *clientConfig) error {
		return nil
	}
}

// WithTraceProvider sets the trace provider for the client.
// If this is not set then the global trace provider is used.
func WithTraceProvider(provider trace.TracerProvider) Opt {
	return WithTraceOptions(otelhttp.WithTracerProvider(provider))
}

// WithTraceOptions sets tracing span options for the client.
func WithTraceOptions(opts ...otelhttp.Option) Opt {
	return func(c *clientConfig) error {
		c.traceOpts = append(c.traceOpts, opts...)
		return nil
	}
}

// WithResponseHook adds a ResponseHook to the client. ResponseHooks are called
// for each HTTP response returned by the daemon. Hooks are invoked in the order
// they were added.
//
// Hooks must not read or close resp.Body.
func WithResponseHook(h ResponseHook) Opt {
	return func(c *clientConfig) error {
		if h == nil {
			return errors.New("invalid response hook: hook is nil")
		}
		c.responseHooks = append(c.responseHooks, h)
		return nil
	}
}
