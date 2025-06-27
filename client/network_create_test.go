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

func TestNetworkCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.NetworkCreate(context.Background(), "mynetwork", network.CreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestNetworkCreateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestNetworkCreateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.NetworkCreate(context.Background(), "mynetwork", network.CreateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestNetworkCreate(t *testing.T) {
	expectedURL := "/networks/create"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			content, err := json.Marshal(network.CreateResponse{
				ID:      "network_id",
				Warning: "warning",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	enableIPv6 := true
	networkResponse, err := client.NetworkCreate(context.Background(), "mynetwork", network.CreateOptions{
		Driver:     "mydriver",
		EnableIPv6: &enableIPv6,
		Internal:   true,
		Options: map[string]string{
			"opt-key": "opt-value",
		},
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(networkResponse.ID, "network_id"))
	assert.Check(t, is.Equal(networkResponse.Warning, "warning"))
}
