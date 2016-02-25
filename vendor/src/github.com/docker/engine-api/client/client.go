package client

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/engine-api/client/transport"
	"github.com/docker/go-connections/tlsconfig"
)

// Client is the API client that performs all operations
// against a docker server.
type Client struct {
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
	// basePath holds the path to prepend to the requests.
	basePath string
	// transport is the interface to sends request with, it implements transport.Client.
	transport transport.Client
	// version of the server to talk to.
	version string
	// custom http headers configured by users.
	customHTTPHeaders map[string]string
	// logging callbacks, if we're meant to log messages
	logger Logger
	// authentication-related callbacks
	authers []interface{}
}

// NewEnvClient initializes a new API client based on environment variables.
// Use DOCKER_HOST to set the url to the docker server.
// Use DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
// Use DOCKER_CERT_PATH to load the tls certificates from.
// Use DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
func NewEnvClient() (*Client, error) {
	var client *http.Client
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		options := tlsconfig.Options{
			CAFile:             filepath.Join(dockerCertPath, "ca.pem"),
			CertFile:           filepath.Join(dockerCertPath, "cert.pem"),
			KeyFile:            filepath.Join(dockerCertPath, "key.pem"),
			InsecureSkipVerify: os.Getenv("DOCKER_TLS_VERIFY") == "",
		}
		tlsc, err := tlsconfig.Client(options)
		if err != nil {
			return nil, err
		}

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsc,
			},
		}
	}

	host := os.Getenv("DOCKER_HOST")
	if host == "" {
		host = DefaultDockerHost
	}
	return NewClient(host, os.Getenv("DOCKER_API_VERSION"), client, nil)
}

// NewClient initializes a new API client for the given host and API version.
// It won't send any version information if the version number is empty.
// It uses the given http client as transport.
// It also initializes the custom http headers to add to each request.
func NewClient(host string, version string, client *http.Client, httpHeaders map[string]string) (*Client, error) {
	proto, addr, basePath, err := ParseHost(host)
	if err != nil {
		return nil, err
	}

	transport, err := transport.NewTransportWithHTTP(proto, addr, client)
	if err != nil {
		return nil, err
	}

	return &Client{
		proto:             proto,
		addr:              addr,
		basePath:          basePath,
		transport:         transport,
		version:           version,
		customHTTPHeaders: httpHeaders,
	}, nil
}

// SetAuth sets callbacks that the library can use to obtain information which
// is needs in order to authenticate to the server.
func (cli *Client) SetAuth(m ...interface{}) {
	cli.authers = m
}

// SetLogger sets a callback that the client can use to log debugging
// messages.  The callback should not treat messages which are passed to it as
// format specifiers.
func (cli *Client) SetLogger(logger Logger) {
	cli.logger = logger
}

// debugf passes debugging messages to the callback, if one is set
func (cli *Client) debugf(format string, args ...interface{}) {
	if cli.logger != nil {
		cli.logger.Debug(fmt.Sprintf(format, args...))
	}
}

// debug passes a debugging message to the callback, if one is set
func (cli *Client) debug(formatted string) {
	if cli.logger != nil {
		cli.logger.Debug(formatted)
	}
}

// infof passes informational messages to the callback, if one is set
func (cli *Client) infof(format string, args ...interface{}) {
	if cli.logger != nil {
		cli.logger.Info(fmt.Sprintf(format, args...))
	}
}

// errorf passes error messages to the right callback, if one is set
func (cli *Client) errorf(format string, args ...interface{}) {
	if cli.logger != nil {
		cli.logger.Error(fmt.Sprintf(format, args...))
	}
}

// getAPIPath returns the versioned request path to call the api.
// It appends the query parameters to the path if they are not empty.
func (cli *Client) getAPIPath(p string, query url.Values) string {
	var apiPath string
	if cli.version != "" {
		v := strings.TrimPrefix(cli.version, "v")
		apiPath = fmt.Sprintf("%s/v%s%s", cli.basePath, v, p)
	} else {
		apiPath = fmt.Sprintf("%s%s", cli.basePath, p)
	}
	if len(query) > 0 {
		apiPath += "?" + query.Encode()
	}
	return apiPath
}

// ClientVersion returns the version string associated with this
// instance of the Client. Note that this value can be changed
// via the DOCKER_API_VERSION env var.
func (cli *Client) ClientVersion() string {
	return cli.version
}

// ParseHost verifies that the given host strings is valid.
func ParseHost(host string) (string, string, string, error) {
	protoAddrParts := strings.SplitN(host, "://", 2)
	if len(protoAddrParts) == 1 {
		return "", "", "", fmt.Errorf("unable to parse docker host `%s`", host)
	}

	var basePath string
	proto, addr := protoAddrParts[0], protoAddrParts[1]
	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return "", "", "", err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return proto, addr, basePath, nil
}
