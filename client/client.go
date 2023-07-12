/*
Package client is a Go client for the Docker Engine API.

For more information about the Engine API, see the documentation:
https://docs.docker.com/engine/api/

# Usage

You use the library by constructing a client object using [NewClientWithOpts]
and calling methods on it. The client can be configured from environment
variables by passing the [FromEnv] option, or configured manually by passing any
of the other available [Opts].

For example, to list running containers (the equivalent of "docker ps"):

	package main

	import (
		"context"
		"fmt"

		"github.com/docker/docker/api/types"
		"github.com/docker/docker/client"
	)

	func main() {
		cli, err := client.NewClientWithOpts(client.FromEnv)
		if err != nil {
			panic(err)
		}

		containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
		if err != nil {
			panic(err)
		}

		for _, container := range containers {
			fmt.Printf("%s %s\n", container.ID[:10], container.Image)
		}
	}
*/
package client // import "github.com/docker/docker/client"

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/go-connections/sockets"
	"github.com/pkg/errors"
)

// DummyHost is a hostname used for local communication.
//
// It acts as a valid formatted hostname for local connections (such as "unix://"
// or "npipe://") which do not require a hostname. It should never be resolved,
// but uses the special-purpose ".localhost" TLD (as defined in [RFC 2606, Section 2]
// and [RFC 6761, Section 6.3]).
//
// [RFC 7230, Section 5.4] defines that an empty header must be used for such
// cases:
//
//	If the authority component is missing or undefined for the target URI,
//	then a client MUST send a Host header field with an empty field-value.
//
// However, [Go stdlib] enforces the semantics of HTTP(S) over TCP, does not
// allow an empty header to be used, and requires req.URL.Scheme to be either
// "http" or "https".
//
// For further details, refer to:
//
//   - https://github.com/docker/engine-api/issues/189
//   - https://github.com/golang/go/issues/13624
//   - https://github.com/golang/go/issues/61076
//   - https://github.com/moby/moby/issues/45935
//
// [RFC 2606, Section 2]: https://www.rfc-editor.org/rfc/rfc2606.html#section-2
// [RFC 6761, Section 6.3]: https://www.rfc-editor.org/rfc/rfc6761#section-6.3
// [RFC 7230, Section 5.4]: https://datatracker.ietf.org/doc/html/rfc7230#section-5.4
// [Go stdlib]: https://github.com/golang/go/blob/6244b1946bc2101b01955468f1be502dbadd6807/src/net/http/transport.go#L558-L569
const DummyHost = "api.moby.localhost"

// ErrRedirect is the error returned by checkRedirect when the request is non-GET.
var ErrRedirect = errors.New("unexpected redirect in response")

// Client is the API client that performs all operations
// against a docker server.
type Client struct {
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
	// custom http headers configured by users.
	customHTTPHeaders map[string]string
	// manualOverride is set to true when the version was set by users.
	manualOverride bool

	// negotiateVersion indicates if the client should automatically negotiate
	// the API version to use when making requests. API version negotiation is
	// performed on the first request, after which negotiated is set to "true"
	// so that subsequent requests do not re-negotiate.
	negotiateVersion bool

	// negotiated indicates that API version negotiation took place
	negotiated bool
}

// CheckRedirect specifies the policy for dealing with redirect responses:
// If the request is non-GET return ErrRedirect, otherwise use the last response.
//
// Go 1.8 changes behavior for HTTP redirects (specifically 301, 307, and 308)
// in the client. The Docker client (and by extension docker API client) can be
// made to send a request like POST /containers//start where what would normally
// be in the name section of the URL is empty. This triggers an HTTP 301 from
// the daemon.
//
// In go 1.8 this 301 will be converted to a GET request, and ends up getting
// a 404 from the daemon. This behavior change manifests in the client in that
// before, the 301 was not followed and the client did not generate an error,
// but now results in a message like Error response from daemon: page not found.
func CheckRedirect(req *http.Request, via []*http.Request) error {
	if via[0].Method == http.MethodGet {
		return http.ErrUseLastResponse
	}
	return ErrRedirect
}

// NewClientWithOpts initializes a new API client with a default HTTPClient, and
// default API host and version. It also initializes the custom HTTP headers to
// add to each request.
//
// It takes an optional list of Opt functional arguments, which are applied in
// the order they're provided, which allows modifying the defaults when creating
// the client. For example, the following initializes a client that configures
// itself with values from environment variables (client.FromEnv), and has
// automatic API version negotiation enabled (client.WithAPIVersionNegotiation()).
//
//	cli, err := client.NewClientWithOpts(
//		client.FromEnv,
//		client.WithAPIVersionNegotiation(),
//	)
func NewClientWithOpts(ops ...Opt) (*Client, error) {
	hostURL, err := ParseHostURL(DefaultDockerHost)
	if err != nil {
		return nil, err
	}

	client, err := defaultHTTPClient(hostURL)
	if err != nil {
		return nil, err
	}
	c := &Client{
		host:    DefaultDockerHost,
		version: api.DefaultVersion,
		client:  client,
		proto:   hostURL.Scheme,
		addr:    hostURL.Host,
	}

	for _, op := range ops {
		if err := op(c); err != nil {
			return nil, err
		}
	}

	if c.scheme == "" {
		c.scheme = "http"

		tlsConfig := resolveTLSConfig(c.client.Transport)
		if tlsConfig != nil {
			// TODO(stevvooe): This isn't really the right way to write clients in Go.
			// `NewClient` should probably only take an `*http.Client` and work from there.
			// Unfortunately, the model of having a host-ish/url-thingy as the connection
			// string has us confusing protocol and transport layers. We continue doing
			// this to avoid breaking existing clients but this should be addressed.
			c.scheme = "https"
		}
	}

	return c, nil
}

func defaultHTTPClient(hostURL *url.URL) (*http.Client, error) {
	transport := &http.Transport{}
	err := sockets.ConfigureTransport(transport, hostURL.Scheme, hostURL.Host)
	if err != nil {
		return nil, err
	}
	return &http.Client{
		Transport:     transport,
		CheckRedirect: CheckRedirect,
	}, nil
}

// Close the transport used by the client
func (cli *Client) Close() error {
	if t, ok := cli.client.Transport.(*http.Transport); ok {
		t.CloseIdleConnections()
	}
	return nil
}

// getAPIPath returns the versioned request path to call the api.
// It appends the query parameters to the path if they are not empty.
func (cli *Client) getAPIPath(ctx context.Context, p string, query url.Values) string {
	var apiPath string
	if cli.negotiateVersion && !cli.negotiated {
		cli.NegotiateAPIVersion(ctx)
	}
	if cli.version != "" {
		v := strings.TrimPrefix(cli.version, "v")
		apiPath = path.Join(cli.basePath, "/v"+v, p)
	} else {
		apiPath = path.Join(cli.basePath, p)
	}
	return (&url.URL{Path: apiPath, RawQuery: query.Encode()}).String()
}

// ClientVersion returns the API version used by this client.
func (cli *Client) ClientVersion() string {
	return cli.version
}

// NegotiateAPIVersion queries the API and updates the version to match the API
// version. NegotiateAPIVersion downgrades the client's API version to match the
// APIVersion if the ping version is lower than the default version. If the API
// version reported by the server is higher than the maximum version supported
// by the client, it uses the client's maximum version.
//
// If a manual override is in place, either through the "DOCKER_API_VERSION"
// (EnvOverrideAPIVersion) environment variable, or if the client is initialized
// with a fixed version (WithVersion(xx)), no negotiation is performed.
//
// If the API server's ping response does not contain an API version, or if the
// client did not get a successful ping response, it assumes it is connected with
// an old daemon that does not support API version negotiation, in which case it
// downgrades to the latest version of the API before version negotiation was
// added (1.24).
func (cli *Client) NegotiateAPIVersion(ctx context.Context) {
	if !cli.manualOverride {
		ping, _ := cli.Ping(ctx)
		cli.negotiateAPIVersionPing(ping)
	}
}

// NegotiateAPIVersionPing downgrades the client's API version to match the
// APIVersion in the ping response. If the API version in pingResponse is higher
// than the maximum version supported by the client, it uses the client's maximum
// version.
//
// If a manual override is in place, either through the "DOCKER_API_VERSION"
// (EnvOverrideAPIVersion) environment variable, or if the client is initialized
// with a fixed version (WithVersion(xx)), no negotiation is performed.
//
// If the API server's ping response does not contain an API version, we assume
// we are connected with an old daemon without API version negotiation support,
// and downgrade to the latest version of the API before version negotiation was
// added (1.24).
func (cli *Client) NegotiateAPIVersionPing(pingResponse types.Ping) {
	if !cli.manualOverride {
		cli.negotiateAPIVersionPing(pingResponse)
	}
}

// negotiateAPIVersionPing queries the API and updates the version to match the
// API version from the ping response.
func (cli *Client) negotiateAPIVersionPing(pingResponse types.Ping) {
	// default to the latest version before versioning headers existed
	if pingResponse.APIVersion == "" {
		pingResponse.APIVersion = "1.24"
	}

	// if the client is not initialized with a version, start with the latest supported version
	if cli.version == "" {
		cli.version = api.DefaultVersion
	}

	// if server version is lower than the client version, downgrade
	if versions.LessThan(pingResponse.APIVersion, cli.version) {
		cli.version = pingResponse.APIVersion
	}

	// Store the results, so that automatic API version negotiation (if enabled)
	// won't be performed on the next request.
	if cli.negotiateVersion {
		cli.negotiated = true
	}
}

// DaemonHost returns the host address used by the client
func (cli *Client) DaemonHost() string {
	return cli.host
}

// HTTPClient returns a copy of the HTTP client bound to the server
func (cli *Client) HTTPClient() *http.Client {
	c := *cli.client
	return &c
}

// ParseHostURL parses a url string, validates the string is a host url, and
// returns the parsed URL
func ParseHostURL(host string) (*url.URL, error) {
	proto, addr, ok := strings.Cut(host, "://")
	if !ok || addr == "" {
		return nil, errors.Errorf("unable to parse docker host `%s`", host)
	}

	var basePath string
	if proto == "tcp" {
		parsed, err := url.Parse("tcp://" + addr)
		if err != nil {
			return nil, err
		}
		addr = parsed.Host
		basePath = parsed.Path
	}
	return &url.URL{
		Scheme: proto,
		Host:   addr,
		Path:   basePath,
	}, nil
}

// Dialer returns a dialer for a raw stream connection, with an HTTP/1.1 header,
// that can be used for proxying the daemon connection.
//
// Used by `docker dial-stdio` (docker/cli#889).
func (cli *Client) Dialer() func(context.Context) (net.Conn, error) {
	return func(ctx context.Context) (net.Conn, error) {
		if transport, ok := cli.client.Transport.(*http.Transport); ok {
			if transport.DialContext != nil && transport.TLSClientConfig == nil {
				return transport.DialContext(ctx, cli.proto, cli.addr)
			}
		}
		return fallbackDial(cli.proto, cli.addr, resolveTLSConfig(cli.client.Transport))
	}
}
