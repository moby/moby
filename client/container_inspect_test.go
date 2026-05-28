package client

import (
	"errors"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerInspectError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerInspect(t.Context(), "nothing", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerInspect(t.Context(), "", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerInspect(t.Context(), "    ", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerInspectContainerNotFound(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerInspect(t.Context(), "unknown", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerInspectWithEmptyID(t *testing.T) {
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	)
	assert.NilError(t, err)

	_, err = client.ContainerInspect(t.Context(), "", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerInspect(t.Context(), "    ", ContainerInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerInspect(t *testing.T) {
	const expectedURL = "/containers/container_id/json"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, container.InspectResponse{
				ID:    "container_id",
				Image: "image",
				Name:  "name",
			})(req)
		}),
	)
	assert.NilError(t, err)

	res, err := client.ContainerInspect(t.Context(), "container_id", ContainerInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.Container.ID, "container_id"))
	assert.Check(t, is.Equal(res.Container.Image, "image"))
	assert.Check(t, is.Equal(res.Container.Name, "name"))
}
