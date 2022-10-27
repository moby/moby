package client

import (
	"context"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
)

// Opt is a configuration option to initialize a client
type Opt func(*Client) error

// FromEnv configures the client with values from environment variables.
//
// FromEnv uses the following environment variables:
//
// DOCKER_HOST (EnvOverrideHost) to set the URL to the docker server.
//
// DOCKER_API_VERSION (EnvOverrideAPIVersion) to set the version of the API to
// use, leave empty for latest.
//
// DOCKER_CERT_PATH (EnvOverrideCertPath) to specify the directory from which to
// load the TLS certificates (ca.pem, cert.pem, key.pem).
//
// DOCKER_TLS_VERIFY (EnvTLSVerify) to enable or disable TLS verification (off by
// default).
func FromEnv(c *Client) error {
	ops := []Opt{
		WithTLSClientConfigFromEnv(),
		WithHostFromEnv(),
		WithVersionFromEnv(),
	}
	for _, op := range ops {
		if err := op(c); err != nil {
			return err
		}
	}
	return nil
}

// WithDialContext applies the dialer to the client transport. This can be
// used to set the Timeout and KeepAlive settings of the client.
func WithDialContext(dialContext func(ctx context.Context, network, addr string) (net.Conn, error)) Opt {
	return func(c *Client) error {
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			transport.DialContext = dialContext
			return nil
		}
		return errors.Errorf("cannot apply dialer to transport: %T", c.client.Transport)
	}
}

// WithHost overrides the client host with the specified one.
func WithHost(host string) Opt {
	return func(c *Client) error {
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
		return errors.Errorf("cannot apply host to transport: %T", c.client.Transport)
	}
}

// WithHostFromEnv overrides the client host with the host specified in the
// DOCKER_HOST (EnvOverrideHost) environment variable. If DOCKER_HOST is not set,
// or set to an empty value, the host is not modified.
func WithHostFromEnv() Opt {
	return func(c *Client) error {
		if host := os.Getenv(EnvOverrideHost); host != "" {
			return WithHost(host)(c)
		}
		return nil
	}
}

// WithHTTPClient overrides the client http client with the specified one
func WithHTTPClient(client *http.Client) Opt {
	return func(c *Client) error {
		if client != nil {
			c.client = client
		}
		return nil
	}
}

// WithTimeout configures the time limit for requests made by the HTTP client
func WithTimeout(timeout time.Duration) Opt {
	return func(c *Client) error {
		c.client.Timeout = timeout
		return nil
	}
}

// WithHTTPHeaders overrides the client default http headers
func WithHTTPHeaders(headers map[string]string) Opt {
	return func(c *Client) error {
		c.customHTTPHeaders = headers
		return nil
	}
}

// WithScheme overrides the client scheme with the specified one
func WithScheme(scheme string) Opt {
	return func(c *Client) error {
		c.scheme = scheme
		return nil
	}
}

// WithTLSClientConfig applies a tls config to the client transport.
func WithTLSClientConfig(cacertPath, certPath, keyPath string) Opt {
	return func(c *Client) error {
		opts := tlsconfig.Options{
			CAFile:             cacertPath,
			CertFile:           certPath,
			KeyFile:            keyPath,
			ExclusiveRootPools: true,
		}
		config, err := tlsconfig.Client(opts)
		if err != nil {
			return errors.Wrap(err, "failed to create tls config")
		}
		if transport, ok := c.client.Transport.(*http.Transport); ok {
			transport.TLSClientConfig = config
			return nil
		}
		return errors.Errorf("cannot apply tls config to transport: %T", c.client.Transport)
	}
}

// WithTLSClientConfigFromEnv configures the client's TLS settings with the
// settings in the DOCKER_CERT_PATH and DOCKER_TLS_VERIFY environment variables.
// If DOCKER_CERT_PATH is not set or empty, TLS configuration is not modified.
//
// WithTLSClientConfigFromEnv uses the following environment variables:
//
// DOCKER_CERT_PATH (EnvOverrideCertPath) to specify the directory from which to
// load the TLS certificates (ca.pem, cert.pem, key.pem).
//
// DOCKER_TLS_VERIFY (EnvTLSVerify) to enable or disable TLS verification (off by
// default).
func WithTLSClientConfigFromEnv() Opt {
	return func(c *Client) error {
		dockerCertPath := os.Getenv(EnvOverrideCertPath)
		if dockerCertPath == "" {
			return nil
		}
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv(EnvTLSVerify) == "",
		}
		tlsc, err := tlsconfig.Client(options)
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

// WithVersion overrides the client version with the specified one. If an empty
// version is specified, the value will be ignored to allow version negotiation.
func WithVersion(version string) Opt {
	return func(c *Client) error {
		if version != "" {
			c.version = version
			c.manualOverride = true
		}
		return nil
	}
}

// WithVersionFromEnv overrides the client version with the version specified in
// the DOCKER_API_VERSION environment variable. If DOCKER_API_VERSION is not set,
// the version is not modified.
func WithVersionFromEnv() Opt {
	return func(c *Client) error {
		return WithVersion(os.Getenv(EnvOverrideAPIVersion))(c)
	}
}

// WithAPIVersionNegotiation enables automatic API version negotiation for the client.
// With this option enabled, the client automatically negotiates the API version
// to use when making requests. API version negotiation is performed on the first
// request; subsequent requests will not re-negotiate.
func WithAPIVersionNegotiation() Opt {
	return func(c *Client) error {
		c.negotiateVersion = true
		return nil
	}
}
