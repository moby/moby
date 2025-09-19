package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.VolumeRemove(context.Background(), "volume_id", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.VolumeRemove(context.Background(), "", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.VolumeRemove(context.Background(), "    ", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestVolumeRemoveConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestVolumeRemoveConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	err = client.VolumeRemove(context.Background(), "volume_id", false)
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestVolumeRemove(t *testing.T) {
	const expectedURL = "/volumes/volume_id"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
			return nil, err
		}
		if v := req.URL.Query().Get("force"); v != "1" {
			return nil, fmt.Errorf("expected force=1, got %s", v)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
		}, nil
	}))
	assert.NilError(t, err)

	err = client.VolumeRemove(context.Background(), "volume_id", true)
	assert.NilError(t, err)
}
