package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerUpdateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerUpdate(context.Background(), "nothing", ContainerUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerUpdate(context.Background(), "", ContainerUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerUpdate(context.Background(), "    ", ContainerUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerUpdate(t *testing.T) {
	const expectedURL = "/containers/container_id/update"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, container.UpdateResponse{})(req)
	}))
	assert.NilError(t, err)

	_, err = client.ContainerUpdate(context.Background(), "container_id", ContainerUpdateOptions{
		Resources: &container.Resources{
			CPUPeriod: 1,
		},
		RestartPolicy: &container.RestartPolicy{
			Name: "always",
		},
	})
	assert.NilError(t, err)
}
