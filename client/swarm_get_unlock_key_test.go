package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmGetUnlockKeyError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmGetUnlockKey(context.Background())
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmGetUnlockKey(t *testing.T) {
	const (
		expectedURL = "/swarm/unlockkey"
		unlockKey   = "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeE"
	)

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, swarm.UnlockKeyResponse{
			UnlockKey: unlockKey,
		})(req)
	}))
	assert.NilError(t, err)

	result, err := client.SwarmGetUnlockKey(context.Background())
	assert.NilError(t, err)
	assert.Check(t, is.Equal(unlockKey, result.Key))
}
