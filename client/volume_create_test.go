package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeCreateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeCreate(context.Background(), VolumeCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestVolumeCreate(t *testing.T) {
	const expectedURL = "/volumes/create"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, volume.Volume{
			Name:       "volume",
			Driver:     "local",
			Mountpoint: "mountpoint",
		})(req)
	}))
	assert.NilError(t, err)

	res, err := client.VolumeCreate(context.Background(), VolumeCreateOptions{
		Name:   "myvolume",
		Driver: "mydriver",
		DriverOpts: map[string]string{
			"opt-key": "opt-value",
		},
	})
	assert.NilError(t, err)
	v := res.Volume
	assert.Check(t, is.Equal(v.Name, "volume"))
	assert.Check(t, is.Equal(v.Driver, "local"))
	assert.Check(t, is.Equal(v.Mountpoint, "mountpoint"))
}
