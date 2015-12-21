package lib

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// NewEnvClient initializes a new API client based on environment variables.
// Use DOCKER_HOST to set the url to the docker server.
// Use DOCKER_API_VERSION to set the version of the API to reach, leave empty for latest.
// Use DOCKER_CERT_PATH to load the tls certificates from.
// Use DOCKER_TLS_VERIFY to enable or disable TLS verification, off by default.
func NewEnvClient() (*Client, error) {
	var transport *http.Transport
	if dockerCertPath := os.Getenv("DOCKER_CERT_PATH"); dockerCertPath != "" {
		tlsc := &tls.Config{}

		cert, err := tls.LoadX509KeyPair(filepath.Join(dockerCertPath, "cert.pem"), filepath.Join(dockerCertPath, "key.pem"))
		if err != nil {
			return nil, fmt.Errorf("Error loading x509 key pair: %s", err)
		}

		tlsc.Certificates = append(tlsc.Certificates, cert)
		tlsc.InsecureSkipVerify = os.Getenv("DOCKER_TLS_VERIFY") == ""
		transport = &http.Transport{
			TLSClientConfig: tlsc,
		}
	}

	return NewClient(os.Getenv("DOCKER_HOST"), os.Getenv("DOCKER_API_VERSION"), transport, nil)
}

// NewClient initializes a new API client for the given host and API version.
// It won't send any version information if the version number is empty.
// It uses the transport to create a new http client.
// It also initializes the custom http headers to add to each request.
func NewClient(host string, version string, transport *http.Transport, httpHeaders map[string]string) (*Client, error) {
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

	transport = configureTransport(transport, proto, addr)
	if transport.TLSClientConfig != nil {
		scheme = "https"
	}

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

func configureTransport(tr *http.Transport, proto, addr string) *http.Transport {
	if tr == nil {
		tr = &http.Transport{}
	}

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
		tr.Dial = (&net.Dialer{Timeout: timeout}).Dial
	}

	return tr
}
