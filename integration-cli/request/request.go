package request // import "github.com/docker/docker/integration-cli/request"

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	dclient "github.com/docker/docker/client"
	"github.com/docker/docker/opts"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/go-connections/sockets"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/pkg/errors"
)

// Method creates a modifier that sets the specified string as the request method
func Method(method string) func(*http.Request) error {
	return func(req *http.Request) error {
		req.Method = method
		return nil
	}
}

// RawString sets the specified string as body for the request
func RawString(content string) func(*http.Request) error {
	return RawContent(ioutil.NopCloser(strings.NewReader(content)))
}

// RawContent sets the specified reader as body for the request
func RawContent(reader io.ReadCloser) func(*http.Request) error {
	return func(req *http.Request) error {
		req.Body = reader
		return nil
	}
}

// ContentType sets the specified Content-Type request header
func ContentType(contentType string) func(*http.Request) error {
	return func(req *http.Request) error {
		req.Header.Set("Content-Type", contentType)
		return nil
	}
}

// JSON sets the Content-Type request header to json
func JSON(req *http.Request) error {
	return ContentType("application/json")(req)
}

// JSONBody creates a modifier that encodes the specified data to a JSON string and set it as request body. It also sets
// the Content-Type header of the request.
func JSONBody(data interface{}) func(*http.Request) error {
	return func(req *http.Request) error {
		jsonData := bytes.NewBuffer(nil)
		if err := json.NewEncoder(jsonData).Encode(data); err != nil {
			return err
		}
		req.Body = ioutil.NopCloser(jsonData)
		req.Header.Set("Content-Type", "application/json")
		return nil
	}
}

// Post creates and execute a POST request on the specified host and endpoint, with the specified request modifiers
func Post(endpoint string, modifiers ...func(*http.Request) error) (*http.Response, io.ReadCloser, error) {
	return Do(endpoint, append(modifiers, Method(http.MethodPost))...)
}

// Delete creates and execute a DELETE request on the specified host and endpoint, with the specified request modifiers
func Delete(endpoint string, modifiers ...func(*http.Request) error) (*http.Response, io.ReadCloser, error) {
	return Do(endpoint, append(modifiers, Method(http.MethodDelete))...)
}

// Get creates and execute a GET request on the specified host and endpoint, with the specified request modifiers
func Get(endpoint string, modifiers ...func(*http.Request) error) (*http.Response, io.ReadCloser, error) {
	return Do(endpoint, modifiers...)
}

// Do creates and execute a request on the specified endpoint, with the specified request modifiers
func Do(endpoint string, modifiers ...func(*http.Request) error) (*http.Response, io.ReadCloser, error) {
	return DoOnHost(DaemonHost(), endpoint, modifiers...)
}

// DoOnHost creates and execute a request on the specified host and endpoint, with the specified request modifiers
func DoOnHost(host, endpoint string, modifiers ...func(*http.Request) error) (*http.Response, io.ReadCloser, error) {
	req, err := newRequest(host, endpoint, modifiers...)
	if err != nil {
		return nil, nil, err
	}
	client, err := newHTTPClient(host)
	if err != nil {
		return nil, nil, err
	}
	resp, err := client.Do(req)
	var body io.ReadCloser
	if resp != nil {
		body = ioutils.NewReadCloserWrapper(resp.Body, func() error {
			defer resp.Body.Close()
			return nil
		})
	}
	return resp, body, err
}

// newRequest creates a new http Request to the specified host and endpoint, with the specified request modifiers
func newRequest(host, endpoint string, modifiers ...func(*http.Request) error) (*http.Request, error) {
	hostUrl, err := dclient.ParseHostURL(host)
	if err != nil {
		return nil, errors.Wrapf(err, "failed parsing url %q", host)
	}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create request")
	}

	if os.Getenv("DOCKER_TLS_VERIFY") != "" {
		req.URL.Scheme = "https"
	} else {
		req.URL.Scheme = "http"
	}
	req.URL.Host = hostUrl.Host

	for _, config := range modifiers {
		if err := config(req); err != nil {
			return nil, err
		}
	}
	return req, nil
}

// newHTTPClient creates an http client for the specific host
// TODO: Share more code with client.defaultHTTPClient
func newHTTPClient(host string) (*http.Client, error) {
	// FIXME(vdemeester) 10*time.Second timeout of SockRequest… ?
	hostUrl, err := dclient.ParseHostURL(host)
	if err != nil {
		return nil, err
	}
	transport := new(http.Transport)
	if hostUrl.Scheme == "tcp" && os.Getenv("DOCKER_TLS_VERIFY") != "" {
		// Setup the socket TLS configuration.
		tlsConfig, err := getTLSConfig()
		if err != nil {
			return nil, err
		}
		transport = &http.Transport{TLSClientConfig: tlsConfig}
	}
	transport.DisableKeepAlives = true
	err = sockets.ConfigureTransport(transport, hostUrl.Scheme, hostUrl.Host)
	return &http.Client{Transport: transport}, err
}

// NewClient returns a new Docker API client
// Deprecated: Use Execution.APIClient()
func NewClient() (dclient.APIClient, error) {
	return dclient.NewClientWithOpts(dclient.WithHost(DaemonHost()))
}

// ReadBody read the specified ReadCloser content and returns it
func ReadBody(b io.ReadCloser) ([]byte, error) {
	defer b.Close()
	return ioutil.ReadAll(b)
}

// SockConn opens a connection on the specified socket
func SockConn(timeout time.Duration, daemon string) (net.Conn, error) {
	daemonURL, err := url.Parse(daemon)
	if err != nil {
		return nil, errors.Wrapf(err, "could not parse url %q", daemon)
	}

	var c net.Conn
	switch daemonURL.Scheme {
	case "npipe":
		return npipeDial(daemonURL.Path, timeout)
	case "unix":
		return net.DialTimeout(daemonURL.Scheme, daemonURL.Path, timeout)
	case "tcp":
		if os.Getenv("DOCKER_TLS_VERIFY") != "" {
			// Setup the socket TLS configuration.
			tlsConfig, err := getTLSConfig()
			if err != nil {
				return nil, err
			}
			dialer := &net.Dialer{Timeout: timeout}
			return tls.DialWithDialer(dialer, daemonURL.Scheme, daemonURL.Host, tlsConfig)
		}
		return net.DialTimeout(daemonURL.Scheme, daemonURL.Host, timeout)
	default:
		return c, errors.Errorf("unknown scheme %v (%s)", daemonURL.Scheme, daemon)
	}
}

func getTLSConfig() (*tls.Config, error) {
	dockerCertPath := os.Getenv("DOCKER_CERT_PATH")

	if dockerCertPath == "" {
		return nil, errors.New("DOCKER_TLS_VERIFY specified, but no DOCKER_CERT_PATH environment variable")
	}

	option := &tlsconfig.Options{
		CAFile:   filepath.Join(dockerCertPath, "ca.pem"),
		CertFile: filepath.Join(dockerCertPath, "cert.pem"),
		KeyFile:  filepath.Join(dockerCertPath, "key.pem"),
	}
	tlsConfig, err := tlsconfig.Client(*option)
	if err != nil {
		return nil, err
	}

	return tlsConfig, nil
}

// DaemonHost return the daemon host string for this test execution
func DaemonHost() string {
	daemonURLStr := "unix://" + opts.DefaultUnixSocket
	if daemonHostVar := os.Getenv("DOCKER_HOST"); daemonHostVar != "" {
		daemonURLStr = daemonHostVar
	}
	return daemonURLStr
}
