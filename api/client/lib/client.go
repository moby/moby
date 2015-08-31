package lib

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/docker/docker/pkg/sockets"
	"github.com/docker/docker/pkg/tlsconfig"
)

// Client is the API client that performs all operations
// against a docker server.
type Client struct {
	// proto holds the client protocol i.e. unix.
	proto string
	// addr holds the client address.
	addr string
	// basePath holds the path to prepend to the requests
	basePath string
	// scheme holds the scheme of the client i.e. https.
	scheme string
	// tlsConfig holds the tls configuration to use in hijacked requests.
	tlsConfig *tls.Config
	// httpClient holds the client transport instance. Exported to keep the old code running.
	httpClient *http.Client
	// version of the server to talk to.
	version string
	// custom http headers configured by users
	customHTTPHeaders map[string]string
}

// NewClient initializes a new API client for the given host and API version.
// It won't send any version information if the version number is empty.
// It uses the tlsOptions to decide whether to use a secure connection or not.
// It also initializes the custom http headers to add to each request.
func NewClient(host string, version string, tlsOptions *tlsconfig.Options, httpHeaders map[string]string) (*Client, error) {
	var (
		basePath       string
		tlsConfig      *tls.Config
		scheme         = "http"
		protoAddrParts = strings.SplitN(host, "://", 2)
		proto, addr    = protoAddrParts[0], protoAddrParts[1]
	)

	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return nil, err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}

	if tlsOptions != nil {
		scheme = "https"
		var err error
		tlsConfig, err = tlsconfig.Client(*tlsOptions)
		if err != nil {
			return nil, err
		}
	}

	// The transport is created here for reuse during the client session.
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}
	sockets.ConfigureTCPTransport(transport, proto, addr)

	return &Client{
		proto:             proto,
		addr:              addr,
		basePath:          basePath,
		scheme:            scheme,
		tlsConfig:         tlsConfig,
		httpClient:        &http.Client{Transport: transport},
		version:           version,
		customHTTPHeaders: httpHeaders,
	}, nil
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
