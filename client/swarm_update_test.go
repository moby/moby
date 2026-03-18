package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmUpdateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmUpdate(t.Context(), SwarmUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmUpdate(t *testing.T) {
	const expectedURL = "/swarm/update"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	_, err = client.SwarmUpdate(t.Context(), SwarmUpdateOptions{})
	assert.NilError(t, err)
}
