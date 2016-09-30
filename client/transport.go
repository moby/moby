package client

import (
	"crypto/tls"
	"errors"
	"net/http"
)

var errTLSConfigUnavailable = errors.New("TLSConfig unavailable")

// transportFunc allows us to inject a mock transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

// resolveTLSConfig attempts to resolve the tls configuration from the
// RoundTripper.
func resolveTLSConfig(transport http.RoundTripper) (*tls.Config, error) {
	switch tr := transport.(type) {
	case *http.Transport:
		return tr.TLSClientConfig, nil
	case transportFunc:
		return nil, nil // detect this type for testing.
	default:
		return nil, errTLSConfigUnavailable
	}
}

// resolveScheme detects a tls config on the transport and returns the
// appropriate http scheme.
//
// TODO(stevvooe): This isn't really the right way to write clients in Go.
// `NewClient` should probably only take an `*http.Client` and work from there.
// Unfortunately, the model of having a host-ish/url-thingy as the connection
// string has us confusing protocol and transport layers. We continue doing
// this to avoid breaking existing clients but this should be addressed.
func resolveScheme(transport http.RoundTripper) (string, error) {
	c, err := resolveTLSConfig(transport)
	if err != nil {
		return "", err
	}

	if c != nil {
		return "https", nil
	}

	return "http", nil
}
