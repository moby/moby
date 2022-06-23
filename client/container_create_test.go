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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestContainerCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, nil, "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error while testing StatusInternalServerError, got %T", err)
	}

	// 404 doesn't automatically means an unknown image
	client, err = NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, nil, "nothing")
	if err == nil || !IsErrNotFound(err) {
		t.Fatalf("expected a Server Error while testing StatusNotFound, got %T", err)
	}
}

func TestContainerCreateImageNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "No such image"))),
	)
	assert.NilError(t, err)
	_, err = client.ContainerCreate(context.Background(), &container.Config{Image: "unknown_image"}, nil, nil, nil, "unknown")
	if err == nil || !IsErrNotFound(err) {
		t.Fatalf("expected an imageNotFound error, got %v, %T", err, err)
	}
}

func TestContainerCreateWithName(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/create"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			name := req.URL.Query().Get("name")
			if name != "container_name" {
				return nil, fmt.Errorf("container name not set in URL query properly. Expected `container_name`, got %s", name)
			}
			b, err := json.Marshal(container.CreateResponse{
				ID: "container_id",
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

	r, err := client.ContainerCreate(context.Background(), nil, nil, nil, nil, "container_name")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "container_id" {
		t.Fatalf("expected `container_id`, got %s", r.ID)
	}
}

// TestContainerCreateAutoRemove validates that a client using API 1.24 always disables AutoRemove. When using API 1.25
// or up, AutoRemove should not be disabled.
func TestContainerCreateAutoRemove(t *testing.T) {
	autoRemoveValidator := func(expectedValue bool) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			var config configWrapper

			if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
				return nil, err
			}
			if config.HostConfig.AutoRemove != expectedValue {
				return nil, fmt.Errorf("expected AutoRemove to be %v, got %v", expectedValue, config.HostConfig.AutoRemove)
			}
			b, err := json.Marshal(container.CreateResponse{
				ID: "container_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}
	}

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(autoRemoveValidator(false))),
		WithVersion("1.24"),
	)
	assert.NilError(t, err)
	if _, err = client.ContainerCreate(context.Background(), nil, &container.HostConfig{AutoRemove: true}, nil, nil, ""); err != nil {
		t.Fatal(err)
	}
	client, err = NewClientWithOpts(
		WithHTTPClient(newMockClient(autoRemoveValidator(true))),
		WithVersion("1.25"),
	)
	assert.NilError(t, err)
	if _, err := client.ContainerCreate(context.Background(), nil, &container.HostConfig{AutoRemove: true}, nil, nil, ""); err != nil {
		t.Fatal(err)
	}
}
