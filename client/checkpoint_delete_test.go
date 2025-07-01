package client

import (
	"bytes"
	"context"
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

func TestCheckpointDeleteError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.CheckpointDelete(context.Background(), "container_id", checkpoint.DeleteOptions{
		CheckpointID: "checkpoint_id",
	})

	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.CheckpointDelete(context.Background(), "", checkpoint.DeleteOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.CheckpointDelete(context.Background(), "    ", checkpoint.DeleteOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestCheckpointDelete(t *testing.T) {
	expectedURL := "/containers/container_id/checkpoints/checkpoint_id"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodDelete {
				return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.CheckpointDelete(context.Background(), "container_id", checkpoint.DeleteOptions{
		CheckpointID: "checkpoint_id",
	})
	assert.NilError(t, err)
}
