package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkDisconnectError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.NetworkDisconnect(context.Background(), "network_id", "container_id", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// Empty network ID or container ID
	err = client.NetworkDisconnect(context.Background(), "", "container_id", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.NetworkDisconnect(context.Background(), "network_id", "", false)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNetworkDisconnect(t *testing.T) {
	const expectedURL = "/networks/network_id/disconnect"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}

		var disconnect network.DisconnectRequest
		if err := json.NewDecoder(req.Body).Decode(&disconnect); err != nil {
			return nil, err
		}

		if disconnect.Container != "container_id" {
			return nil, fmt.Errorf("expected 'container_id', got %s", disconnect.Container)
		}

		if !disconnect.Force {
			return nil, fmt.Errorf("expected Force to be true, got %v", disconnect.Force)
		}

		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	err = client.NetworkDisconnect(context.Background(), "network_id", "container_id", true)
	assert.NilError(t, err)
}
