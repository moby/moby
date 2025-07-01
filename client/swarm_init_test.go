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

func TestSwarmInitError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.SwarmInit(context.Background(), swarm.InitRequest{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmInit(t *testing.T) {
	expectedURL := "/swarm/init"

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
				Body:       io.NopCloser(bytes.NewReader([]byte(`"body"`))),
			}, nil
		}),
	}

	resp, err := client.SwarmInit(context.Background(), swarm.InitRequest{
		ListenAddr: "0.0.0.0:2377",
	})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp, "body"))
}
