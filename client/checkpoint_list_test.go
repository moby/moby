package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/checkpoint"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointListError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "container_id", CheckpointListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestCheckpointList(t *testing.T) {
	const expectedURL = "/containers/container_id/checkpoints"

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, []checkpoint.Summary{
				{Name: "checkpoint"},
			})(req)
		}),
	)
	assert.NilError(t, err)

	res, err := client.CheckpointList(context.Background(), "container_id", CheckpointListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Len(res.Checkpoints, 1))
}

func TestCheckpointListContainerNotFound(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "unknown", CheckpointListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}
