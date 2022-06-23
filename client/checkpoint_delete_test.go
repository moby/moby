package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
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

func TestCheckpointDeleteError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	err = client.CheckpointDelete(context.Background(), "container_id", types.CheckpointDeleteOptions{
		CheckpointID: "checkpoint_id",
	})

	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestCheckpointDelete(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/checkpoints/checkpoint_id"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
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
		})),
	)
	assert.NilError(t, err)

	err = client.CheckpointDelete(context.Background(), "container_id", types.CheckpointDeleteOptions{
		CheckpointID: "checkpoint_id",
	})

	if err != nil {
		t.Fatal(err)
	}
}
