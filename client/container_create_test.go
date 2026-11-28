package client

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/mount"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerCreateError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: nil, Name: "nothing"})
	assert.Error(t, err, "config.Image or Image is required")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))

	_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{}, Name: "nothing"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
}

func TestContainerCreateImageNotFound(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "No such image")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "unknown_image"}, Name: "unknown"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerCreateWithName(t *testing.T) {
	const expectedURL = "/containers/create"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			name := req.URL.Query().Get("name")
			if name != "container_name" {
				return nil, fmt.Errorf("container name not set in URL query properly. Expected `container_name`, got %s", name)
			}
			return mockJSONResponse(http.StatusOK, nil, container.CreateResponse{
				ID: "container_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	r, err := client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}, Name: "container_name"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "container_id"))
}

func TestContainerCreateAutoRemove(t *testing.T) {
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			var config container.CreateRequest
			if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
				return nil, err
			}
			if !config.HostConfig.AutoRemove {
				return nil, errors.New("expected AutoRemove to be enabled")
			}
			return mockJSONResponse(http.StatusOK, nil, container.CreateResponse{
				ID: "container_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	resp, err := client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}, HostConfig: &container.HostConfig{AutoRemove: true}})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.ID, "container_id"))
}

// TestContainerCreateIDMappingRequiresVersion verifies that an id-mapped bind
// mount is refused client-side when the daemon's API version predates it,
// rather than being sent to a daemon that would silently ignore the option.
func TestContainerCreateIDMappingRequiresVersion(t *testing.T) {
	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{{
			Type:   mount.TypeBind,
			Source: "/src",
			Target: "/dst",
			BindOptions: &mount.BindOptions{
				IDMapping: &mount.IDMapping{Source: mount.IDMappingSourceMatchUser},
			},
		}},
	}

	t.Run("old API version", func(t *testing.T) {
		var requests int
		client, err := New(
			WithAPIVersion("1.56"),
			WithMockClient(func(req *http.Request) (*http.Response, error) {
				requests++
				return mockJSONResponse(http.StatusOK, nil, container.CreateResponse{ID: "container_id"})(req)
			}),
		)
		assert.NilError(t, err)

		_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}, HostConfig: hostConfig})
		assert.ErrorContains(t, err, "BindOptions.IDMapping")
		assert.ErrorContains(t, err, "requires API version 1.57")
		assert.Check(t, is.Equal(requests, 0))
	})

	t.Run("supported API version", func(t *testing.T) {
		client, err := New(
			WithAPIVersion("1.57"),
			WithMockClient(mockJSONResponse(http.StatusOK, nil, container.CreateResponse{ID: "container_id"})),
		)
		assert.NilError(t, err)

		resp, err := client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}, HostConfig: hostConfig})
		assert.NilError(t, err)
		assert.Check(t, is.Equal(resp.ID, "container_id"))
	})
}

// TestContainerCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestContainerCreateConnectionError(t *testing.T) {
	client, err := New(WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}})
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

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			var config container.CreateRequest

			if err := json.NewDecoder(req.Body).Decode(&config); err != nil {
				return nil, err
			}
			assert.Check(t, is.DeepEqual(config.HostConfig.CapAdd, expectedCaps))
			assert.Check(t, is.DeepEqual(config.HostConfig.CapDrop, expectedCaps))

			return mockJSONResponse(http.StatusOK, nil, container.CreateResponse{
				ID: "container_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.ContainerCreate(t.Context(), ContainerCreateOptions{Config: &container.Config{Image: "test"}, HostConfig: &container.HostConfig{CapAdd: inputCaps, CapDrop: inputCaps}})
	assert.NilError(t, err)
}
