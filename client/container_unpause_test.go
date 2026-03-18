package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerUnpauseError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerUnpause(t.Context(), "nothing", ContainerUnpauseOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerUnpause(t.Context(), "", ContainerUnpauseOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerUnpause(t.Context(), "    ", ContainerUnpauseOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerUnpause(t *testing.T) {
	const expectedURL = "/containers/container_id/unpause"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)
	_, err = client.ContainerUnpause(t.Context(), "container_id", ContainerUnpauseOptions{})
	assert.NilError(t, err)
}
