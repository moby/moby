package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/containerd/errdefs"
	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestExecCreateError(t *testing.T) {
	client, err := New(
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
	client, err := New(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ExecCreate(context.Background(), "container_id", ExecCreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestExecCreate(t *testing.T) {
	const expectedURL = "/containers/container_id/exec"
	client, err := New(
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
			return mockJSONResponse(http.StatusOK, nil, container.ExecCreateResponse{
				ID: "exec_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	res, err := client.ExecCreate(context.Background(), "container_id", ExecCreateOptions{
		User: "user",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.ID, "exec_id"))
}

func TestExecStartError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ExecStart(context.Background(), "nothing", ExecStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestExecStart(t *testing.T) {
	const expectedURL = "/exec/exec_id/start"
	client, err := New(
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
			return mockResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.ExecStart(context.Background(), "exec_id", ExecStartOptions{
		Detach: true,
		TTY:    false,
	})
	assert.NilError(t, err)
}

func TestExecStartConsoleSize(t *testing.T) {
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, nil
		}),
	)
	assert.NilError(t, err)

	_, err = client.ExecStart(context.Background(), "exec_id", ExecStartOptions{
		Detach:      true,
		TTY:         false,
		ConsoleSize: ConsoleSize{Height: 100, Width: 100},
	})
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "console size is only supported when TTY is enabled"))
}

func TestExecInspectError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ExecInspect(context.Background(), "nothing", ExecInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestExecInspect(t *testing.T) {
	const expectedURL = "/exec/exec_id/json"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, container.ExecInspectResponse{
				ID:          "exec_id",
				ContainerID: "container_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	inspect, err := client.ExecInspect(context.Background(), "exec_id", ExecInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.ID, "exec_id"))
	assert.Check(t, is.Equal(inspect.ContainerID, "container_id"))
}
