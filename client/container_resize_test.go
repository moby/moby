package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
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

func TestContainerResizeError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	err = client.ContainerResize(context.Background(), "container_id", types.ResizeOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerExecResizeError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	err = client.ContainerExecResize(context.Background(), "exec_id", types.ResizeOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerResize(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(resizeTransport("/v" + api.DefaultVersion + "/containers/container_id/resize"))),
	)
	assert.NilError(t, err)

	err = client.ContainerResize(context.Background(), "container_id", types.ResizeOptions{
		Height: 500,
		Width:  600,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestContainerExecResize(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(resizeTransport("/v" + api.DefaultVersion + "/exec/exec_id/resize"))),
	)
	assert.NilError(t, err)

	err = client.ContainerExecResize(context.Background(), "exec_id", types.ResizeOptions{
		Height: 500,
		Width:  600,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func resizeTransport(expectedURL string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(req.URL.Path, expectedURL) {
			return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
		}
		query := req.URL.Query()
		h := query.Get("h")
		if h != "500" {
			return nil, fmt.Errorf("h not set in URL query properly. Expected '500', got %s", h)
		}
		w := query.Get("w")
		if w != "600" {
			return nil, fmt.Errorf("w not set in URL query properly. Expected '600', got %s", w)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}
}
