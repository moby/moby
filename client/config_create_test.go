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

func TestConfigCreateUnsupported(t *testing.T) {
	client := &Client{
		version: "1.29",
		client:  &http.Client{},
	}
	_, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.Check(t, is.Error(err, `"config create" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestConfigCreateError(t *testing.T) {
	client := &Client{
		version: "1.30",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestConfigCreate(t *testing.T) {
	expectedURL := "/v1.30/configs/create"
	client := &Client{
		version: "1.30",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
	}

	r, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "test_config"))
}
