package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client/transport"
)

type mockClient struct {
	do func(*http.Request) (*http.Response, error)
}

// TLSConfig returns the TLS configuration.
func (m *mockClient) TLSConfig() *tls.Config {
	return &tls.Config{}
}

// Scheme returns protocol scheme to use.
func (m *mockClient) Scheme() string {
	return "http"
}

// Secure returns true if there is a TLS configuration.
func (m *mockClient) Secure() bool {
	return false
}

// NewMockClient returns a mocked client that runs the function supplied as `client.Do` call
func newMockClient(tlsConfig *tls.Config, doer func(*http.Request) (*http.Response, error)) transport.Client {
	if tlsConfig != nil {
		panic("this actually gets set!")
	}

	return &mockClient{
		do: doer,
	}
}

// Do executes the supplied function for the mock.
func (m mockClient) Do(req *http.Request) (*http.Response, error) {
	return m.do(req)
}

func errorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		header := http.Header{}
		header.Set("Content-Type", "application/json")

		body, err := json.Marshal(&types.ErrorResponse{
			Message: message,
		})
		if err != nil {
			return nil, err
		}

		return &http.Response{
			StatusCode: statusCode,
			Body:       ioutil.NopCloser(bytes.NewReader(body)),
			Header:     header,
		}, nil
	}
}

func plainTextErrorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(message))),
		}, nil
	}
}
