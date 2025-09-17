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

func TestContainerRestartError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	err = client.ContainerRestart(context.Background(), "nothing", ContainerStopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerRestart(context.Background(), "", ContainerStopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerRestart(context.Background(), "    ", ContainerStopOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestContainerRestartConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerRestartConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	err = client.ContainerRestart(context.Background(), "nothing", ContainerStopOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestContainerRestart(t *testing.T) {
	const expectedURL = "/containers/container_id/restart"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		s := req.URL.Query().Get("signal")
		if s != "SIGKILL" {
			return nil, fmt.Errorf("signal not set in URL query. Expected 'SIGKILL', got '%s'", s)
		}
		t := req.URL.Query().Get("t")
		if t != "100" {
			return nil, fmt.Errorf("t (timeout) not set in URL query properly. Expected '100', got %s", t)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}))
	assert.NilError(t, err)
	timeout := 100
	err = client.ContainerRestart(context.Background(), "container_id", ContainerStopOptions{
		Signal:  "SIGKILL",
		Timeout: &timeout,
	})
	assert.NilError(t, err)
}
