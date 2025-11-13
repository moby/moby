package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/common"
	"github.com/moby/moby/api/types/swarm"
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

func assertRequestWithQuery(req *http.Request, expMethod string, expectedPath string, expectedQuery string) error {
	if err := assertRequest(req, expMethod, expectedPath); err != nil {
		return err
	}
	q := req.URL.Query().Encode()
	if q != expectedQuery {
		return fmt.Errorf("expected query '%s', got '%s'", expectedQuery, q)
	}
	return nil
}

// ensureBody makes sure the response has a Body (using [http.NoBody] if
// none is present), and that the request is set on the response, then returns
// it as a testRoundTripper.
func ensureBody(f func(req *http.Request) (*http.Response, error)) testRoundTripper {
	return func(req *http.Request) (*http.Response, error) {
		resp, err := f(req)
		if resp != nil {
			if resp.Body == nil {
				resp.Body = http.NoBody
			}
			if resp.Request == nil {
				resp.Request = req
			}
		}
		return resp, err
	}
}

// makeTestRoundTripper makes sure the response has a Body, using [http.NoBody] if
// none is present, and returns it as a testRoundTripper. If withDefaults is set,
// it also mocks the "/_ping" endpoint and sets default headers as returned
// by the daemon.
func makeTestRoundTripper(f func(req *http.Request) (*http.Response, error)) testRoundTripper {
	return func(req *http.Request) (*http.Response, error) {
		if req.URL.Path == "/_ping" {
			return mockPingResponse(http.StatusOK, PingResult{
				APIVersion:     MaxAPIVersion,
				OSType:         runtime.GOOS,
				Experimental:   true,
				BuilderVersion: build.BuilderBuildKit,
				SwarmStatus: &SwarmStatus{
					NodeState:        swarm.LocalNodeStateActive,
					ControlAvailable: true,
				},
			})(req)
		}
		resp, err := f(req)
		if resp != nil {
			if resp.Body == nil {
				resp.Body = http.NoBody
			}
			if resp.Request == nil {
				resp.Request = req
			}
		}
		applyDefaultHeaders(resp)
		return resp, err
	}
}

// applyDefaultHeaders mocks the headers set by the daemon's VersionMiddleware.
func applyDefaultHeaders(resp *http.Response) {
	if resp == nil {
		return
	}
	if resp.Header == nil {
		resp.Header = make(http.Header)
	}
	if resp.Header.Get("Server") == "" {
		resp.Header.Set("Server", fmt.Sprintf("Docker/%s (%s)", "v99.99.99", runtime.GOOS))
	}
	if resp.Header.Get("Api-Version") == "" {
		resp.Header.Set("Api-Version", MaxAPIVersion)
	}
	if resp.Header.Get("Ostype") == "" {
		resp.Header.Set("Ostype", runtime.GOOS)
	}
}

// WithMockClient is a test helper that allows you to inject a mock client for
// testing. By default, it mocks the "/_ping" endpoint, so allow the client
// to perform API-version negotiation. Other endpoints are handled by "doer".
func WithMockClient(doer func(*http.Request) (*http.Response, error)) Opt {
	return WithHTTPClient(&http.Client{
		Transport: makeTestRoundTripper(doer),
	})
}

// WithBaseMockClient is a test helper that allows you to inject a mock client
// for testing. It is identical to [WithMockClient], but does not mock the "/_ping"
// endpoint, and doesn't set the default headers.
func WithBaseMockClient(doer func(*http.Request) (*http.Response, error)) Opt {
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

// mockPingResponse mocks the headers set for a "/_ping" response.
func mockPingResponse(statusCode int, ping PingResult) func(req *http.Request) (*http.Response, error) {
	headers := http.Header{}
	if s := ping.SwarmStatus; s != nil {
		role := "worker"
		if s.ControlAvailable {
			role = "manager"
		}
		headers.Set("Swarm", fmt.Sprintf("%s/%s", string(swarm.LocalNodeStateActive), role))
	}
	headers.Set("Api-Version", ping.APIVersion)
	headers.Set("Ostype", ping.OSType)
	headers.Set("Docker-Experimental", strconv.FormatBool(ping.Experimental))
	headers.Set("Builder-Version", string(ping.BuilderVersion))

	headers.Set("Content-Type", "text/plain; charset=utf-8")
	headers.Set("Cache-Control", "no-cache, no-store, must-revalidate")
	headers.Set("Pragma", "no-cache")
	return mockResponse(statusCode, headers, "OK")
}

func mockResponse(statusCode int, headers http.Header, respBody string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		var body io.ReadCloser
		if respBody == "" || req.Method == http.MethodHead {
			body = http.NoBody
		} else {
			body = io.NopCloser(strings.NewReader(respBody))
		}
		return &http.Response{
			Status:     fmt.Sprintf("%d %s", statusCode, http.StatusText(statusCode)),
			StatusCode: statusCode,
			Header:     headers.Clone(),
			Body:       body,
			Request:    req,
		}, nil
	}
}
