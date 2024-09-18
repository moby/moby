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

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSwarmInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.SwarmInspect(context.Background())
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestSwarmInspect(t *testing.T) {
	expectedURL := "/swarm"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(swarm.Swarm{
				ClusterInfo: swarm.ClusterInfo{
					ID: "swarm_id",
				},
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

	swarmInspect, err := client.SwarmInspect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if swarmInspect.ID != "swarm_id" {
		t.Fatalf("expected `swarm_id`, got %s", swarmInspect.ID)
	}
}
