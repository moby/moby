package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmInspectError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmInspect(context.Background())
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmInspect(t *testing.T) {
	const expectedURL = "/swarm"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		content, err := json.Marshal(swarm.Swarm{
			ClusterInfo: swarm.ClusterInfo{
				ID: "swarm_id",
			},
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(content)),
		}, nil
	}))
	assert.NilError(t, err)

	swarmInspect, err := client.SwarmInspect(context.Background())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(swarmInspect.ID, "swarm_id"))
}
