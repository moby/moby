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
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestContainerExecCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerExecCreate(context.Background(), "container_id", types.ExecConfig{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerExecCreate(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/exec"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
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
			execConfig := &types.ExecConfig{}
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
		})),
	)
	assert.NilError(t, err)

	r, err := client.ContainerExecCreate(context.Background(), "container_id", types.ExecConfig{
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
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	err = client.ContainerExecStart(context.Background(), "nothing", types.ExecStartCheck{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerExecStart(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/exec/exec_id/start"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if err := req.ParseForm(); err != nil {
				return nil, err
			}
			execStartCheck := &types.ExecStartCheck{}
			if err := json.NewDecoder(req.Body).Decode(execStartCheck); err != nil {
				return nil, err
			}
			if execStartCheck.Tty || !execStartCheck.Detach {
				return nil, fmt.Errorf("expected execStartCheck{Detach:true,Tty:false}, got %v", execStartCheck)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		})),
	)
	assert.NilError(t, err)

	err = client.ContainerExecStart(context.Background(), "exec_id", types.ExecStartCheck{
		Detach: true,
		Tty:    false,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContainerExecInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerExecInspect(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerExecInspect(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/exec/exec_id/json"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			b, err := json.Marshal(types.ContainerExecInspect{
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
		})),
	)
	assert.NilError(t, err)

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
