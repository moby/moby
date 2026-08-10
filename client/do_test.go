package client

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestDo verifies that Do completes the request like any other client
// request: versioned path, pass-through of method, query, headers and body,
// and an open response body on success.
func TestDo(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequestWithQuery(req, http.MethodPost, "/custom/endpoint", "key=value"); err != nil {
			return nil, err
		}
		if hdr := req.Header.Get("X-Custom"); hdr != "custom-value" {
			return errorMock(http.StatusInternalServerError, "unexpected X-Custom header: "+hdr)(req)
		}
		b, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		if string(b) != "request-body" {
			return errorMock(http.StatusInternalServerError, "unexpected body: "+string(b))(req)
		}
		return mockResponse(http.StatusOK, nil, "response-body")(req)
	}))
	assert.NilError(t, err)

	resp, err := client.Do(t.Context(), http.MethodPost, "/custom/endpoint",
		url.Values{"key": {"value"}},
		strings.NewReader("request-body"),
		http.Header{"X-Custom": {"custom-value"}},
	)
	assert.NilError(t, err)
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(resp.Body)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(b), "response-body"))
}

// TestDoError verifies that a non-2xx response is turned into a typed error,
// consistent with the other methods of the client.
func TestDoError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusNotFound, "no such endpoint")))
	assert.NilError(t, err)

	_, err = client.Do(t.Context(), http.MethodGet, "/custom/endpoint", nil, nil, nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
	assert.Check(t, is.ErrorContains(err, "no such endpoint"))
}
