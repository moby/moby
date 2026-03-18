package client

import (
	"errors"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeInspectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeInspect(t.Context(), "nothing", VolumeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestVolumeInspectNotFound(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeInspect(t.Context(), "unknown", VolumeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestVolumeInspectWithEmptyID(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.VolumeInspect(t.Context(), "", VolumeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.VolumeInspect(t.Context(), "    ", VolumeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestVolumeInspect(t *testing.T) {
	const expectedURL = "/volumes/volume_id"
	expected := volume.Volume{
		Name:       "name",
		Driver:     "driver",
		Mountpoint: "mountpoint",
	}

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, expected)(req)
	}))
	assert.NilError(t, err)

	result, err := client.VolumeInspect(t.Context(), "volume_id", VolumeInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, result.Volume))
}
