package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceInspectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ServiceInspect(context.Background(), "nothing", ServiceInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestServiceInspectServiceNotFound(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.ServiceInspect(context.Background(), "unknown", ServiceInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestServiceInspectWithEmptyID(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.ServiceInspect(context.Background(), "", ServiceInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ServiceInspect(context.Background(), "    ", ServiceInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestServiceInspect(t *testing.T) {
	const expectedURL = "/services/service_id"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, swarm.Service{
			ID: "service_id",
		})(req)
	}))
	assert.NilError(t, err)

	inspect, err := client.ServiceInspect(context.Background(), "service_id", ServiceInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.Service.ID, "service_id"))
}
