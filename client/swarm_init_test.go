package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmInitError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmInit(context.Background(), SwarmInitOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmInit(t *testing.T) {
	const expectedURL = "/swarm/init"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, "node-id")(req)
	}))
	assert.NilError(t, err)

	result, err := client.SwarmInit(context.Background(), SwarmInitOptions{
		ListenAddr: "0.0.0.0:2377",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(result.NodeID, "node-id"))
}
