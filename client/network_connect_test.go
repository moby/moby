package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkConnectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.NetworkConnect(context.Background(), "network_id", "container_id", nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	// Empty network ID or container ID
	err = client.NetworkConnect(context.Background(), "", "container_id", nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.NetworkConnect(context.Background(), "network_id", "", nil)
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNetworkConnectEmptyNilEndpointSettings(t *testing.T) {
	expectedURL := "/networks/network_id/connect"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			var connect network.ConnectOptions
			if err := json.NewDecoder(req.Body).Decode(&connect); err != nil {
				return nil, err
			}

			if connect.Container != "container_id" {
				return nil, fmt.Errorf("expected 'container_id', got %s", connect.Container)
			}

			if connect.EndpointConfig != nil {
				return nil, fmt.Errorf("expected connect.EndpointConfig to be nil, got %v", connect.EndpointConfig)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.NetworkConnect(context.Background(), "network_id", "container_id", nil)
	assert.NilError(t, err)
}

func TestNetworkConnect(t *testing.T) {
	expectedURL := "/networks/network_id/connect"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			var connect network.ConnectOptions
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

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.NetworkConnect(context.Background(), "network_id", "container_id", &network.EndpointSettings{
		NetworkID: "NetworkID",
	})
	assert.NilError(t, err)
}
