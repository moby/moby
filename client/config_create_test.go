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
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestConfigCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.30"),
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestConfigCreate(t *testing.T) {
	expectedURL := "/v1.30/configs/create"
	client, err := NewClientWithOpts(
		WithVersion("1.30"),
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			b, err := json.Marshal(swarm.ConfigCreateResponse{
				ID: "test_config",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	r, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "test_config"))
}
