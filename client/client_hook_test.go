package client

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"gotest.tools/v3/assert"
)

func TestHookTransportRequest(t *testing.T) {
	const (
		hdrKey  = "X-Test-Header"
		hdrVal  = "hello-world"
		bodyVal = "request body"
	)

	req, err := http.NewRequest(http.MethodPost, "http://example.com", strings.NewReader(bodyVal))
	assert.NilError(t, err)

	originalHeader := req.Header

	tr := &hookTransport{
		reqHooks: []RequestHook{
			func(req *http.Request) error {
				req.Header.Set(hdrKey, hdrVal)

				_, err := io.ReadAll(req.Body)
				assert.Error(t, err, "hooks must not read HTTP message body")
				assert.NilError(t, req.Body.Close())

				body, err := req.GetBody()
				assert.NilError(t, err)
				_, err = io.ReadAll(body)
				assert.Error(t, err, "hooks must not read HTTP message body")

				return nil
			},
		},
		base: roundTripperFunc(func(got *http.Request) (*http.Response, error) {
			assert.Check(t, got != req)
			assert.Equal(t, got.Header.Get(hdrKey), hdrVal)

			body, err := io.ReadAll(got.Body)
			assert.NilError(t, err)
			assert.Equal(t, string(body), bodyVal)

			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body:       http.NoBody,
				Request:    got,
			}, nil
		}),
	}

	_, err = tr.RoundTrip(req)
	assert.NilError(t, err)

	assert.Equal(t, originalHeader.Get(hdrKey), "")
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
