// +build test

package transport

import (
	"crypto/tls"
	"net/http"
)

type mockClient struct {
	*tlsInfo
	do func(*http.Request) (*http.Response, error)
}

// NewMockClient returns a mocked client that runs the function supplied as `client.Do` call
func NewMockClient(tlsConfig *tls.Config, doer func(*http.Request) (*http.Response, error)) Client {
	return mockClient{
		tlsInfo: &tlsInfo{tlsConfig},
		do:      doer,
	}
}

// Do executes the supplied function for the mock.
func (m mockClient) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}
