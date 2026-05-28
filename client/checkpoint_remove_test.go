package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointDeleteError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointRemove(t.Context(), "container_id", CheckpointRemoveOptions{
		CheckpointID: "checkpoint_id",
	})

	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.CheckpointRemove(t.Context(), "", CheckpointRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.CheckpointRemove(t.Context(), "    ", CheckpointRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCheckpointDelete(t *testing.T) {
	const expectedURL = "/containers/container_id/checkpoints/checkpoint_id"

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
				return nil, err
			}
			return mockResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointRemove(t.Context(), "container_id", CheckpointRemoveOptions{
		CheckpointID: "checkpoint_id",
	})
	assert.NilError(t, err)
}
