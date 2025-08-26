package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceUpdateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ServiceUpdate(context.Background(), "service_id", swarm.Version{}, swarm.ServiceSpec{}, ServiceUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ServiceUpdate(context.Background(), "", swarm.Version{}, swarm.ServiceSpec{}, ServiceUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ServiceUpdate(context.Background(), "    ", swarm.Version{}, swarm.ServiceSpec{}, ServiceUpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

// TestServiceUpdateConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestServiceUpdateConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ServiceUpdate(context.Background(), "service_id", swarm.Version{}, swarm.ServiceSpec{}, ServiceUpdateOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestServiceUpdate(t *testing.T) {
	expectedURL := "/services/service_id/update"

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
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
			}),
		}

		_, err := client.ServiceUpdate(context.Background(), "service_id", updateCase.swarmVersion, swarm.ServiceSpec{}, ServiceUpdateOptions{})
		assert.NilError(t, err)
	}
}
