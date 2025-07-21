package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/checkpoint"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.CheckpointCreate(context.Background(), "nothing", checkpoint.CreateOptions{
		CheckpointID: "noting",
		Exit:         true,
	})

	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.CheckpointCreate(context.Background(), "", checkpoint.CreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.CheckpointCreate(context.Background(), "    ", checkpoint.CreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCheckpointCreate(t *testing.T) {
	expectedContainerID := "container_id"
	expectedCheckpointID := "checkpoint_id"
	expectedURL := "/containers/container_id/checkpoints"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			createOptions := &checkpoint.CreateOptions{}
			if err := json.NewDecoder(req.Body).Decode(createOptions); err != nil {
				return nil, err
			}

			if createOptions.CheckpointID != expectedCheckpointID {
				return nil, fmt.Errorf("expected CheckpointID to be 'checkpoint_id', got %v", createOptions.CheckpointID)
			}

			if !createOptions.Exit {
				return nil, errors.New("expected Exit to be true")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.CheckpointCreate(context.Background(), expectedContainerID, checkpoint.CreateOptions{
		CheckpointID: expectedCheckpointID,
		Exit:         true,
	})
	assert.NilError(t, err)
}
