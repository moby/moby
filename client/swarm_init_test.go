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
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestSwarmInitError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.SwarmInit(context.Background(), swarm.InitRequest{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSwarmInit(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/swarm/init"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`"body"`))),
			}, nil
		})),
	)
	assert.NilError(t, err)

	resp, err := client.SwarmInit(context.Background(), swarm.InitRequest{
		ListenAddr: "0.0.0.0:2377",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp != "body" {
		t.Fatalf("Expected 'body', got %s", resp)
	}
}
