package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerKillError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	err = client.ContainerKill(context.Background(), "nothing", "SIGKILL")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerKill(context.Background(), "", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerKill(context.Background(), "    ", "")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerKill(t *testing.T) {
	const expectedURL = "/containers/container_id/kill"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
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
	)
	assert.NilError(t, err)

	err = client.ContainerKill(context.Background(), "container_id", "SIGKILL")
	assert.NilError(t, err)
}
