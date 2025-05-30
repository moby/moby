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
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerKillError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerKill(context.Background(), "nothing", "SIGKILL")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerKill(context.Background(), "", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerKill(context.Background(), "    ", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerKill(t *testing.T) {
	expectedURL := "/containers/container_id/kill"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			signal := req.URL.Query().Get("signal")
			if signal != "SIGKILL" {
				return nil, fmt.Errorf("signal not set in URL query properly. Expected 'SIGKILL', got %s", signal)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.ContainerKill(context.Background(), "container_id", "SIGKILL")
	assert.NilError(t, err)
}
