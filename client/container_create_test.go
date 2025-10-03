package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), nil, nil, nil, nil, "nothing")
	assert.Error(t, err, "config is nil")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))

	_, err = client.ContainerCreate(context.Background(), &container.Config{}, nil, nil, nil, "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// 404 doesn't automatically means an unknown image
	client, err = NewClientWithOpts(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), &container.Config{}, nil, nil, nil, "nothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerCreateImageNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusNotFound, "No such image")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), &container.Config{Image: "unknown_image"}, nil, nil, nil, "unknown")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerCreateWithName(t *testing.T) {
	const expectedURL = "/containers/create"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
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
	)
	assert.NilError(t, err)

	r, err := client.ContainerCreate(context.Background(), &container.Config{}, nil, nil, nil, "container_name")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "container_id"))
}

func TestContainerCreateAutoRemove(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			var config container.CreateRequest
			if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
				return nil, err
			}
			if !config.HostConfig.AutoRemove {
				return nil, errors.New("expected AutoRemove to be enabled")
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
	)
	assert.NilError(t, err)

	resp, err := client.ContainerCreate(context.Background(), &container.Config{}, &container.HostConfig{AutoRemove: true}, nil, nil, "")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, "container_id"))
}

// TestContainerCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), &container.Config{}, nil, nil, nil, "")
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

	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
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
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(context.Background(), &container.Config{}, &container.HostConfig{CapAdd: inputCaps, CapDrop: inputCaps}, nil, nil, "")
	assert.NilError(t, err)
}
