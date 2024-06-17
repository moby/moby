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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerExecCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerExecCreate(context.Background(), "container_id", container.ExecOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

// TestContainerExecCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerExecCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ContainerExecCreate(context.Background(), "", container.ExecOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestContainerExecCreate(t *testing.T) {
	expectedURL := "/containers/container_id/exec"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
			execConfig := &container.ExecOptions{}
			if err := json.NewDecoder(req.Body).Decode(execConfig); err != nil {
				return nil, err
			}
			if execConfig.User != "user" {
				return nil, fmt.Errorf("expected an execConfig with User == 'user', got %v", execConfig)
			}
			b, err := json.Marshal(types.IDResponse{
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
	}

	r, err := client.ContainerExecCreate(context.Background(), "container_id", container.ExecOptions{
		User: "user",
	})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "exec_id" {
		t.Fatalf("expected `exec_id`, got %s", r.ID)
	}
}

func TestContainerExecStartError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerExecStart(context.Background(), "nothing", container.ExecStartOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerExecStart(t *testing.T) {
	expectedURL := "/exec/exec_id/start"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			options := &container.ExecStartOptions{}
			if err := json.NewDecoder(req.Body).Decode(options); err != nil {
				return nil, err
			}
			if options.Tty || !options.Detach {
				return nil, fmt.Errorf("expected ExecStartOptions{Detach:true,Tty:false}, got %v", options)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.ContainerExecStart(context.Background(), "exec_id", container.ExecStartOptions{
		Detach: true,
		Tty:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContainerExecInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerExecInspect(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerExecInspect(t *testing.T) {
	expectedURL := "/exec/exec_id/json"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
	}

	inspect, err := client.ContainerExecInspect(context.Background(), "exec_id")
	if err != nil {
		t.Fatal(err)
	}
	if inspect.ExecID != "exec_id" {
		t.Fatalf("expected ExecID to be `exec_id`, got %s", inspect.ExecID)
	}
	if inspect.ContainerID != "container_id" {
		t.Fatalf("expected ContainerID `container_id`, got %s", inspect.ContainerID)
	}
}
