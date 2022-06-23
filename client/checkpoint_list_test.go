package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestCheckpointListError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "container_id", types.CheckpointListOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestCheckpointList(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/checkpoints"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal([]types.Checkpoint{
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
		})),
	)
	assert.NilError(t, err)

	checkpoints, err := client.CheckpointList(context.Background(), "container_id", types.CheckpointListOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 1 {
		t.Fatalf("expected 1 checkpoint, got %v", checkpoints)
	}
}

func TestCheckpointListContainerNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.CheckpointList(context.Background(), "unknown", types.CheckpointListOptions{})
	if err == nil || !IsErrNotFound(err) {
		t.Fatalf("expected a containerNotFound error, got %v", err)
	}
}
