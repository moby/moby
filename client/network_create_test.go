package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkCreateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NetworkCreate(context.Background(), "mynetwork", NetworkCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestNetworkCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestNetworkCreateConnectionError(t *testing.T) {
	client, err := New(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.NetworkCreate(context.Background(), "mynetwork", NetworkCreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestNetworkCreate(t *testing.T) {
	const expectedURL = "/networks/create"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, network.CreateResponse{
			ID:      "network_id",
			Warning: "warning",
		})(req)
	}))
	assert.NilError(t, err)

	enableIPv6 := true
	networkResponse, err := client.NetworkCreate(context.Background(), "mynetwork", NetworkCreateOptions{
		Driver:     "mydriver",
		EnableIPv6: &enableIPv6,
		Internal:   true,
		Options: map[string]string{
			"opt-key": "opt-value",
		},
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(networkResponse.ID, "network_id"))
	assert.Check(t, is.Len(networkResponse.Warning, 1))
	assert.Check(t, is.Equal(networkResponse.Warning[0], "warning"))
}
