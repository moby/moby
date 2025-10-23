package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/moby/moby/api/types/common"
)

// defaultAPIPath is the API path prefix for the default API version used.
const defaultAPIPath = "/v" + MaxAPIVersion

// assertRequest checks for the request method and path. If the expected
// path does not contain a version prefix, it is prefixed with the current API
// version.
func assertRequest(req *http.Request, expMethod string, expectedPath string) error {
	if !strings.HasPrefix(expectedPath, "/v1.") {
		expectedPath = defaultAPIPath + expectedPath
	}
	if !strings.HasPrefix(req.URL.Path, expectedPath) {
		return fmt.Errorf("expected URL '%s', got '%s'", expectedPath, req.URL.Path)
	}
	if req.Method != expMethod {
		return fmt.Errorf("expected %s method, got %s", expMethod, req.Method)
	}
	return nil
}

// ensureBody makes sure the response has a Body, using [http.NoBody] if
// none is present, and returns it as a testRoundTripper.
func ensureBody(f func(req *http.Request) (*http.Response, error)) testRoundTripper {
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
	return WithHTTPClient(&http.Client{
		Transport: ensureBody(doer),
	})
}

func errorMock(statusCode int, message string) func(req *http.Request) (*http.Response, error) {
	return mockJSONResponse(statusCode, nil, common.ErrorResponse{
		Message: message,
	})
}

func mockJSONResponse[T any](statusCode int, headers http.Header, resp T) func(req *http.Request) (*http.Response, error) {
	respBody, err := json.Marshal(&resp)
	if err != nil {
		panic(err)
	}
	hdr := make(http.Header)
	if headers != nil {
		hdr = headers.Clone()
	}
	hdr.Set("Content-Type", "application/json")
	return mockResponse(statusCode, hdr, string(respBody))
}

func mockResponse(statusCode int, headers http.Header, respBody string) func(req *http.Request) (*http.Response, error) {
	if headers == nil {
		headers = make(http.Header)
	}
	var body io.ReadCloser
	if respBody == "" {
		body = http.NoBody
	} else {
		body = io.NopCloser(strings.NewReader(respBody))
	}
	return func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
			StatusCode: statusCode,
			Header:     headers,
			Body:       body,
			Request:    req,
		}, nil
	}
}
