package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/moby/moby/api/types/common"
)

// transportFunc allows us to inject a mock transport for testing. We define it
// here so we can detect the tlsconfig and return nil for only this type.
type transportFunc func(*http.Request) (*http.Response, error)

func (tf transportFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return tf(req)
}

func transportEnsureBody(f transportFunc) transportFunc {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := f(req)
		if resp != nil && resp.Body == nil {
			resp.Body = http.NoBody
		}
		return resp, err
	}
}

// WithMockClient is a test helper that allows you to inject a mock client for testing.
func WithMockClient(doer func(*http.Request) (*http.Response, error)) Opt {
	return func(c *clientConfig) error {
		c.client = &http.Client{
			Transport: transportEnsureBody(transportFunc(doer)),
		}
		if !c.manualOverride {
			c.version = ""
		}
		c.host = ""
		c.proto = ""
		c.addr = ""
		return nil
	}
}

func errorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		header := http.Header{}
		header.Set("Content-Type", "application/json")

		body, err := json.Marshal(&common.ErrorResponse{
			Message: message,
		})
		if err != nil {
			return nil, err
		}

		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewReader(body)),
			Header:     header,
		}, nil
	}
}

func plainTextErrorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: statusCode,
			Body:       io.NopCloser(bytes.NewReader([]byte(message))),
		}, nil
	}
}
