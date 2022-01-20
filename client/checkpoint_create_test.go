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

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestCheckpointCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.CheckpointCreate(context.Background(), "nothing", types.CheckpointCreateOptions{
		CheckpointID: "noting",
		Exit:         true,
	})

	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestCheckpointCreate(t *testing.T) {
	expectedContainerID := "container_id"
	expectedCheckpointID := "checkpoint_id"
	expectedURL := "/containers/container_id/checkpoints"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			createOptions := &types.CheckpointCreateOptions{}
			if err := json.NewDecoder(req.Body).Decode(createOptions); err != nil {
				return nil, err
			}

			if createOptions.CheckpointID != expectedCheckpointID {
				return nil, fmt.Errorf("expected CheckpointID to be 'checkpoint_id', got %v", createOptions.CheckpointID)
			}

			if !createOptions.Exit {
				return nil, fmt.Errorf("expected Exit to be true")
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.CheckpointCreate(context.Background(), expectedContainerID, types.CheckpointCreateOptions{
		CheckpointID: expectedCheckpointID,
		Exit:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
}
