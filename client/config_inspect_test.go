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

func TestConfigInspectNotFound(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigInspect(context.Background(), "unknown", ConfigInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestConfigInspectWithEmptyID(t *testing.T) {
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	)
	assert.NilError(t, err)
	_, err = client.ConfigInspect(context.Background(), "", ConfigInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ConfigInspect(context.Background(), "    ", ConfigInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestConfigInspectError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigInspect(context.Background(), "nothing", ConfigInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestConfigInspectConfigNotFound(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusNotFound, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigInspect(context.Background(), "unknown", ConfigInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestConfigInspect(t *testing.T) {
	const expectedURL = "/configs/config_id"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, swarm.Config{
				ID: "config_id",
			})(req)
		}),
	)
	assert.NilError(t, err)

	result, err := client.ConfigInspect(context.Background(), "config_id", ConfigInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(result.Config.ID, "config_id"))
}
