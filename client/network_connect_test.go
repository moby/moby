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

func TestNetworkConnectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NetworkConnect(context.Background(), "network_id", NetworkConnectOptions{
		Container: "container_id",
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// Empty network ID or container ID
	_, err = client.NetworkConnect(context.Background(), "", NetworkConnectOptions{
		Container: "container_id",
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.NetworkConnect(context.Background(), "network_id", NetworkConnectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNetworkConnectEmptyNilEndpointSettings(t *testing.T) {
	const expectedURL = "/networks/network_id/connect"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}

		var connect network.ConnectRequest
		if err := json.NewDecoder(req.Body).Decode(&connect); err != nil {
			return nil, err
		}

		if connect.Container != "container_id" {
			return nil, fmt.Errorf("expected 'container_id', got %s", connect.Container)
		}

		if connect.EndpointConfig != nil {
			return nil, fmt.Errorf("expected connect.EndpointConfig to be nil, got %v", connect.EndpointConfig)
		}

		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	_, err = client.NetworkConnect(context.Background(), "network_id", NetworkConnectOptions{
		Container: "container_id",
	})
	assert.NilError(t, err)
}

func TestNetworkConnect(t *testing.T) {
	const expectedURL = "/networks/network_id/connect"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}

		var connect network.ConnectRequest
		if err := json.NewDecoder(req.Body).Decode(&connect); err != nil {
			return nil, err
		}

		if connect.Container != "container_id" {
			return nil, fmt.Errorf("expected 'container_id', got %s", connect.Container)
		}

		if connect.EndpointConfig == nil {
			return nil, fmt.Errorf("expected connect.EndpointConfig to be not nil, got %v", connect.EndpointConfig)
		}

		if connect.EndpointConfig.NetworkID != "NetworkID" {
			return nil, fmt.Errorf("expected 'NetworkID', got %s", connect.EndpointConfig.NetworkID)
		}

		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	_, err = client.NetworkConnect(context.Background(), "network_id", NetworkConnectOptions{
		Container: "container_id",
		EndpointConfig: &network.EndpointSettings{
			NetworkID: "NetworkID",
		},
	})
	assert.NilError(t, err)
}
