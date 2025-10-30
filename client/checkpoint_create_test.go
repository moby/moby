package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointCreateError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	err = client.CheckpointCreate(context.Background(), "nothing", CheckpointCreateOptions{
		CheckpointID: "noting",
		Exit:         true,
	})

	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.CheckpointCreate(context.Background(), "", CheckpointCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.CheckpointCreate(context.Background(), "    ", CheckpointCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCheckpointCreate(t *testing.T) {
	const (
		expectedContainerID  = "container_id"
		expectedCheckpointID = "checkpoint_id"
		expectedURL          = "/containers/container_id/checkpoints"
	)

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}

			createOptions := &CheckpointCreateOptions{}
			if err := json.NewDecoder(req.Body).Decode(createOptions); err != nil {
				return nil, err
			}

			if createOptions.CheckpointID != expectedCheckpointID {
				return nil, fmt.Errorf("expected CheckpointID to be 'checkpoint_id', got %v", createOptions.CheckpointID)
			}

			if !createOptions.Exit {
				return nil, errors.New("expected Exit to be true")
			}
			return mockJSONResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	err = client.CheckpointCreate(context.Background(), expectedContainerID, CheckpointCreateOptions{
		CheckpointID: expectedCheckpointID,
		Exit:         true,
	})
	assert.NilError(t, err)
}
