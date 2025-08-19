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

func TestContainerCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerCreate(context.Background(), nil, nil, nil, nil, "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// 404 doesn't automatically means an unknown image
	client = &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}
	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, nil, "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerCreateImageNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "No such image")),
	}
	_, err := client.ContainerCreate(context.Background(), &container.Config{Image: "unknown_image"}, nil, nil, nil, "unknown")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerCreateWithName(t *testing.T) {
	expectedURL := "/containers/create"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
		}),
	}

	r, err := client.ContainerCreate(context.Background(), nil, nil, nil, nil, "container_name")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "container_id"))
}

// TestContainerCreateAutoRemove validates that a client using API 1.24 always disables AutoRemove. When using API 1.25
// or up, AutoRemove should not be disabled.
func TestContainerCreateAutoRemove(t *testing.T) {
	autoRemoveValidator := func(expectedValue bool) func(req *http.Request) (*http.Response, error) {
		return func(req *http.Request) (*http.Response, error) {
			var config container.CreateRequest

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
	testCases := []struct {
		version            string
		expectedAutoRemove bool
	}{
		{version: "1.24", expectedAutoRemove: false},
		{version: "1.25", expectedAutoRemove: true},
	}
	for _, tc := range testCases {
		t.Run(tc.version, func(t *testing.T) {
			client := &Client{
				client:  newMockClient(autoRemoveValidator(tc.expectedAutoRemove)),
				version: tc.version,
			}
			_, err := client.ContainerCreate(context.Background(), nil, &container.HostConfig{AutoRemove: true}, nil, nil, "")
			assert.NilError(t, err)
		})
	}
}

// TestContainerCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, nil, "")
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

// TestContainerCreateCapabilities verifies that CapAdd and CapDrop capabilities
// are normalized to their canonical form.
func TestContainerCreateCapabilities(t *testing.T) {
	inputCaps := []string{
		"all",
		"ALL",
		"capability_b",
		"capability_a",
		"capability_c",
		"CAPABILITY_D",
		"CAP_CAPABILITY_D",
	}

	expectedCaps := []string{
		"ALL",
		"CAP_CAPABILITY_A",
		"CAP_CAPABILITY_B",
		"CAP_CAPABILITY_C",
		"CAP_CAPABILITY_D",
	}

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			var config container.CreateRequest

			if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
				return nil, err
			}
			assert.Check(t, is.DeepEqual(config.HostConfig.CapAdd, expectedCaps))
			assert.Check(t, is.DeepEqual(config.HostConfig.CapDrop, expectedCaps))

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
		}),
		version: "1.24",
	}

	_, err := client.ContainerCreate(context.Background(), nil, &container.HostConfig{CapAdd: inputCaps, CapDrop: inputCaps}, nil, nil, "")
	assert.NilError(t, err)
}
