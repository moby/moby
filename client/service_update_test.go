package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestServiceUpdateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.ServiceUpdate(context.Background(), "service_id", swarm.Version{}, swarm.ServiceSpec{}, types.ServiceUpdateOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestServiceUpdate(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/services/service_id/update"

	updateCases := []struct {
		swarmVersion    swarm.Version
		expectedVersion string
	}{
		{
			expectedVersion: "0",
		},
		{
			swarmVersion: swarm.Version{
				Index: 0,
			},
			expectedVersion: "0",
		},
		{
			swarmVersion: swarm.Version{
				Index: 10,
			},
			expectedVersion: "10",
		},
	}

	for _, updateCase := range updateCases {
		client, err := NewClientWithOpts(
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				if req.Method != http.MethodPost {
					return nil, fmt.Errorf("expected POST method, got %s", req.Method)
				}
				version := req.URL.Query().Get("version")
				if version != updateCase.expectedVersion {
					return nil, fmt.Errorf("version not set in URL query properly, expected '%s', got %s", updateCase.expectedVersion, version)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte("{}"))),
				}, nil
			})),
		)
		assert.NilError(t, err)

		_, err = client.ServiceUpdate(context.Background(), "service_id", updateCase.swarmVersion, swarm.ServiceSpec{}, types.ServiceUpdateOptions{})
		if err != nil {
			t.Fatal(err)
		}
	}
}
