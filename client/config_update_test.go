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

func TestConfigUpdateUnsupported(t *testing.T) {
	client := &Client{
		version: "1.29",
		client:  &http.Client{},
	}
	err := client.ConfigUpdate(context.Background(), "config_id", swarm.Version{}, swarm.ConfigSpec{})
	assert.Check(t, is.Error(err, `"config update" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestConfigUpdateError(t *testing.T) {
	client := &Client{
		version: "1.30",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.ConfigUpdate(context.Background(), "config_id", swarm.Version{}, swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ConfigUpdate(context.Background(), "", swarm.Version{}, swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ConfigUpdate(context.Background(), "    ", swarm.Version{}, swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestConfigUpdate(t *testing.T) {
	expectedURL := "/v1.30/configs/config_id/update"

	client := &Client{
		version: "1.30",
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.ConfigUpdate(context.Background(), "config_id", swarm.Version{}, swarm.ConfigSpec{})
	assert.NilError(t, err)
}
