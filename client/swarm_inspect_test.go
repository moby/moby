package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmInspectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmInspect(t.Context(), SwarmInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmInspect(t *testing.T) {
	const expectedURL = "/swarm"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, swarm.Swarm{
			ClusterInfo: swarm.ClusterInfo{
				ID: "swarm_id",
			},
		})(req)
	}))
	assert.NilError(t, err)

	res, err := client.SwarmInspect(t.Context(), SwarmInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.Swarm.ID, "swarm_id"))
}
