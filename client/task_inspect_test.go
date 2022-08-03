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
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

func TestTaskInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, _, err = client.TaskInspectWithRaw(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestTaskInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, _, err = client.TaskInspectWithRaw(context.Background(), "")
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestTaskInspect(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/tasks/task_id"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(swarm.Task{
				ID: "task_id",
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

	taskInspect, _, err := client.TaskInspectWithRaw(context.Background(), "task_id")
	if err != nil {
		t.Fatal(err)
	}
	if taskInspect.ID != "task_id" {
		t.Fatalf("expected `task_id`, got %s", taskInspect.ID)
	}
}
