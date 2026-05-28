package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerPauseError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerPause(t.Context(), "nothing", ContainerPauseOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerPause(t *testing.T) {
	const expectedURL = "/containers/container_id/pause"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			return mockResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.ContainerPause(t.Context(), "container_id", ContainerPauseOptions{})
	assert.NilError(t, err)
}
