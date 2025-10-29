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
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NetworkDisconnect(context.Background(), "network_id", NetworkDisconnectOptions{
		Container: "container_id",
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// Empty network ID or container ID
	_, err = client.NetworkDisconnect(context.Background(), "", NetworkDisconnectOptions{
		Container: "container_id",
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.NetworkDisconnect(context.Background(), "network_id", NetworkDisconnectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNetworkDisconnect(t *testing.T) {
	const expectedURL = "/networks/network_id/disconnect"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
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

	_, err = client.NetworkDisconnect(context.Background(), "network_id", NetworkDisconnectOptions{Container: "container_id", Force: true})
	assert.NilError(t, err)
}
