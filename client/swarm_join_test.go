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

func TestSwarmJoinError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.SwarmJoin(context.Background(), swarm.JoinRequest{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmJoin(t *testing.T) {
	expectedURL := "/swarm/join"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
	}))
	assert.NilError(t, err)

	err = client.SwarmJoin(context.Background(), swarm.JoinRequest{
		ListenAddr: "0.0.0.0:2377",
	})
	assert.NilError(t, err)
}
