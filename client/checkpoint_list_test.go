package client

import (
	"bytes"
	"context"
	"encoding/json"
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

func TestCheckpointListError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "container_id", CheckpointListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestCheckpointList(t *testing.T) {
	expectedURL := "/containers/container_id/checkpoints"

	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal([]checkpoint.Summary{
				{
					Name: "checkpoint",
				},
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	checkpoints, err := client.CheckpointList(context.Background(), "container_id", CheckpointListOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Len(checkpoints, 1))
}

func TestCheckpointListContainerNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "unknown", CheckpointListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}
