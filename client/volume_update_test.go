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
	volumetypes "github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeUpdateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.VolumeUpdate(context.Background(), "volume", swarm.Version{}, volumetypes.UpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.VolumeUpdate(context.Background(), "", swarm.Version{}, volumetypes.UpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.VolumeUpdate(context.Background(), "    ", swarm.Version{}, volumetypes.UpdateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestVolumeUpdate(t *testing.T) {
	expectedURL := "/volumes/test1"
	expectedVersion := "version=10"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPut {
				return nil, fmt.Errorf("expected PUT method, got %s", req.Method)
			}
			if !strings.Contains(req.URL.RawQuery, expectedVersion) {
				return nil, fmt.Errorf("expected query to contain '%s', got '%s'", expectedVersion, req.URL.RawQuery)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	}

	err := client.VolumeUpdate(context.Background(), "test1", swarm.Version{Index: uint64(10)}, volumetypes.UpdateOptions{})
	assert.NilError(t, err)
}
