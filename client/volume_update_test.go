package client

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeUpdateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeUpdate(t.Context(), "volume", VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.VolumeUpdate(t.Context(), "", VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.VolumeUpdate(t.Context(), "    ", VolumeUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestVolumeUpdate(t *testing.T) {
	const (
		expectedURL     = "/volumes/test1"
		expectedVersion = "version=10"
	)

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPut, expectedURL); err != nil {
			return nil, err
		}
		if !strings.Contains(req.URL.RawQuery, expectedVersion) {
			return nil, fmt.Errorf("expected query to contain '%s', got '%s'", expectedVersion, req.URL.RawQuery)
		}
		return mockResponse(http.StatusOK, nil, "body")(req)
	}))
	assert.NilError(t, err)

	_, err = client.VolumeUpdate(t.Context(), "test1", VolumeUpdateOptions{
		Version: swarm.Version{Index: uint64(10)},
	})
	assert.NilError(t, err)
}
