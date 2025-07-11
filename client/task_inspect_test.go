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
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestTaskInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, _, err := client.TaskInspectWithRaw(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestTaskInspectWithEmptyID(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	}
	_, _, err := client.TaskInspectWithRaw(context.Background(), "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, _, err = client.TaskInspectWithRaw(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestTaskInspect(t *testing.T) {
	expectedURL := "/tasks/task_id"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
		}),
	}

	taskInspect, _, err := client.TaskInspectWithRaw(context.Background(), "task_id")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(taskInspect.ID, "task_id"))
}
