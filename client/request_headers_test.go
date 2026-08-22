package client

import (
	"net/http"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

// TestWithRequestHeaders verifies that headers carried by the request context
// are set on the outgoing request, merged with the client's configured
// headers, and scoped to the context they were attached to.
func TestWithRequestHeaders(t *testing.T) {
	var gotHeaders http.Header
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			gotHeaders = req.Header.Clone()
			return mockResponse(http.StatusOK, nil, "[]")(req)
		}),
		WithHTTPHeaders(map[string]string{"X-Client-Scoped": "instance"}),
	)
	assert.NilError(t, err)

	ctx := WithRequestHeaders(t.Context(), http.Header{
		"X-Request-Scoped": {"request"},
	})
	_, err = client.ContainerList(ctx, ContainerListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(gotHeaders.Get("X-Request-Scoped"), "request"))
	assert.Check(t, is.Equal(gotHeaders.Get("X-Client-Scoped"), "instance"))

	// A request sent with a context without extra headers must not carry them.
	_, err = client.ContainerList(t.Context(), ContainerListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(gotHeaders.Get("X-Request-Scoped"), ""))
	assert.Check(t, is.Equal(gotHeaders.Get("X-Client-Scoped"), "instance"))
}

// TestWithRequestHeadersPrecedence verifies that context headers cannot
// override headers the client sets for a specific request.
func TestWithRequestHeadersPrecedence(t *testing.T) {
	var gotHeaders http.Header
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		gotHeaders = req.Header.Clone()
		return mockResponse(http.StatusOK, nil, "OK")(req)
	}))
	assert.NilError(t, err)

	ctx := WithRequestHeaders(t.Context(), http.Header{
		"Content-Type": {"text/plain"},
	})
	// checkpointCreate is an arbitrary call sending a JSON body, for which
	// the client sets Content-Type itself.
	_, err = client.CheckpointCreate(ctx, "nosuchcontainer", CheckpointCreateOptions{CheckpointID: "checkpoint"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(gotHeaders.Get("Content-Type"), "application/json"))
}
