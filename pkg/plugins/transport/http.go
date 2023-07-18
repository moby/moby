package transport // import "github.com/docker/docker/pkg/plugins/transport"

import (
	"io"
	"net/http"
	"strings"
)

// httpTransport holds an http.RoundTripper
// and information about the scheme and address the transport
// sends request to.
type httpTransport struct {
	http.RoundTripper
	scheme string
	addr   string
}

// NewHTTPTransport creates a new httpTransport.
func NewHTTPTransport(r http.RoundTripper, scheme, addr string) Transport {
	return httpTransport{
		RoundTripper: r,
		scheme:       scheme,
		addr:         addr,
	}
}

// NewRequest creates a new http.Request and sets the URL
// scheme and address with the transport's fields.
func (t httpTransport) NewRequest(path string, data io.Reader) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	req, err := http.NewRequest(http.MethodPost, path, data)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept", VersionMimetype)
	req.URL.Scheme = t.scheme
	req.URL.Host = t.addr
	return req, nil
}
