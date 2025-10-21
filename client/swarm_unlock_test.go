package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmUnlockError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmUnlock(context.Background(), SwarmUnlockOptions{Key: "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeU"})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmUnlock(t *testing.T) {
	const expectedURL = "/swarm/unlock"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}))
	assert.NilError(t, err)

	_, err = client.SwarmUnlock(context.Background(), SwarmUnlockOptions{Key: "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeU"})
	assert.NilError(t, err)
}
