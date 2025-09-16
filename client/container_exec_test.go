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
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerExecCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerExecCreate(context.Background(), "container_id", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerExecCreate(context.Background(), "", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerExecCreate(context.Background(), "    ", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestContainerExecCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerExecCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ContainerExecCreate(context.Background(), "container_id", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestContainerExecCreate(t *testing.T) {
	expectedURL := "/containers/container_id/exec"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			// FIXME validate the content is the given ExecConfig ?
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			execConfig := &container.ExecCreateRequest{}
			if err := json.NewDecoder(req.Body).Decode(execConfig); err != nil {
				return nil, err
			}
			if execConfig.User != "user" {
				return nil, fmt.Errorf("expected an execConfig with User == 'user', got %v", execConfig)
			}
			b, err := json.Marshal(container.ExecCreateResponse{
				ID: "exec_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	r, err := client.ContainerExecCreate(context.Background(), "container_id", ExecCreateOptions{
		User: "user",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "exec_id"))
}

func TestContainerExecStartError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	err = client.ContainerExecStart(context.Background(), "nothing", ExecStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerExecStart(t *testing.T) {
	expectedURL := "/exec/exec_id/start"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			request := &container.ExecStartRequest{}
			if err := json.NewDecoder(req.Body).Decode(request); err != nil {
				return nil, err
			}
			if request.Tty || !request.Detach {
				return nil, fmt.Errorf("expected ExecStartOptions{Detach:true,Tty:false}, got %v", request)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	)
	assert.NilError(t, err)

	err = client.ContainerExecStart(context.Background(), "exec_id", ExecStartOptions{
		Detach: true,
		Tty:    false,
	})
	assert.NilError(t, err)
}

func TestContainerExecInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerExecInspect(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerExecInspect(t *testing.T) {
	expectedURL := "/exec/exec_id/json"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			b, err := json.Marshal(container.ExecInspect{
				ExecID:      "exec_id",
				ContainerID: "container_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	inspect, err := client.ContainerExecInspect(context.Background(), "exec_id")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.ExecID, "exec_id"))
	assert.Check(t, is.Equal(inspect.ContainerID, "container_id"))
}
