package client

import (
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeRemoveError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeRemove(t.Context(), "volume_id", VolumeRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.VolumeRemove(t.Context(), "", VolumeRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.VolumeRemove(t.Context(), "    ", VolumeRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestVolumeRemoveConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestVolumeRemoveConnectionError(t *testing.T) {
	client, err := New(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.VolumeRemove(t.Context(), "volume_id", VolumeRemoveOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestVolumeRemove(t *testing.T) {
	const expectedURL = "/volumes/volume_id"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
			return nil, err
		}
		if v := req.URL.Query().Get("force"); v != "1" {
			return nil, fmt.Errorf("expected force=1, got %s", v)
		}

		return mockResponse(http.StatusOK, nil, "body")(req)
	}))
	assert.NilError(t, err)

	_, err = client.VolumeRemove(t.Context(), "volume_id", VolumeRemoveOptions{Force: true})
	assert.NilError(t, err)
}
