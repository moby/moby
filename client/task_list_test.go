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
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestTaskListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.TaskList(context.Background(), TaskListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestTaskList(t *testing.T) {
	const expectedURL = "/tasks"

	listCases := []struct {
		options             TaskListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: TaskListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: TaskListOptions{
				Filters: filters.NewArgs(
					filters.Arg("label", "label1"),
					filters.Arg("label", "label2"),
				),
			},
			expectedQueryParams: map[string]string{
				"filters": `{"label":{"label1":true,"label2":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				for key, expected := range listCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				content, err := json.Marshal([]swarm.Task{
					{
						ID: "task_id1",
					},
					{
						ID: "task_id2",
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
		}

		tasks, err := client.TaskList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(tasks, 2))
	}
}
