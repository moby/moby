package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestNetworkCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.NetworkCreate(context.Background(), "mynetwork", types.NetworkCreate{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestNetworkCreate(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/networks/create"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			content, err := json.Marshal(types.NetworkCreateResponse{
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
		})),
	)
	assert.NilError(t, err)

	networkResponse, err := client.NetworkCreate(context.Background(), "mynetwork", types.NetworkCreate{
		CheckDuplicate: true,
		Driver:         "mydriver",
		EnableIPv6:     true,
		Internal:       true,
		Options: map[string]string{
			"opt-key": "opt-value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if networkResponse.ID != "network_id" {
		t.Fatalf("expected networkResponse.ID to be 'network_id', got %s", networkResponse.ID)
	}
	if networkResponse.Warning != "warning" {
		t.Fatalf("expected networkResponse.Warning to be 'warning', got %s", networkResponse.Warning)
	}
}
