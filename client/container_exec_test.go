package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestExecCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ExecCreate(context.Background(), "container_id", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ExecCreate(context.Background(), "", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ExecCreate(context.Background(), "    ", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestExecCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestExecCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ExecCreate(context.Background(), "container_id", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestExecCreate(t *testing.T) {
	const expectedURL = "/containers/container_id/exec"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
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

	r, err := client.ExecCreate(context.Background(), "container_id", ExecCreateOptions{
		User: "user",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "exec_id"))
}

func TestExecStartError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ExecStart(context.Background(), "nothing", ExecStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestExecStart(t *testing.T) {
	const expectedURL = "/exec/exec_id/start"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
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

	_, err = client.ExecStart(context.Background(), "exec_id", ExecStartOptions{
		Detach: true,
		Tty:    false,
	})
	assert.NilError(t, err)
}

func TestExecInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ExecInspect(context.Background(), "nothing", ExecInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestExecInspect(t *testing.T) {
	const expectedURL = "/exec/exec_id/json"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			b, err := json.Marshal(container.ExecInspectResponse{
				ID:          "exec_id",
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

	inspect, err := client.ExecInspect(context.Background(), "exec_id", ExecInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.ExecID, "exec_id"))
	assert.Check(t, is.Equal(inspect.ContainerID, "container_id"))
}
