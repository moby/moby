package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointDeleteError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	err = client.CheckpointDelete(context.Background(), "container_id", CheckpointDeleteOptions{
		CheckpointID: "checkpoint_id",
	})

	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.CheckpointDelete(context.Background(), "", CheckpointDeleteOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.CheckpointDelete(context.Background(), "    ", CheckpointDeleteOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCheckpointDelete(t *testing.T) {
	const expectedURL = "/containers/container_id/checkpoints/checkpoint_id"

	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	)
	assert.NilError(t, err)

	err = client.CheckpointDelete(context.Background(), "container_id", CheckpointDeleteOptions{
		CheckpointID: "checkpoint_id",
	})
	assert.NilError(t, err)
}
