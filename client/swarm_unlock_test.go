package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
)

func TestSwarmUnlockError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.SwarmUnlock(context.Background(), swarm.UnlockRequest{UnlockKey: "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeU"})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSwarmUnlock(t *testing.T) {
	expectedURL := "/swarm/unlock"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.SwarmUnlock(context.Background(), swarm.UnlockRequest{UnlockKey: "SWMKEY-1-y6guTZNTwpQeTL5RhUfOsdBdXoQjiB2GADHSRJvbXeU"})
	if err != nil {
		t.Fatal(err)
	}
}
