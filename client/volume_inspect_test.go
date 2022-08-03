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

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.VolumeInspect(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestVolumeInspectNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.VolumeInspect(context.Background(), "unknown")
	assert.Check(t, IsErrNotFound(err))
}

func TestVolumeInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, _, err = client.VolumeInspectWithRaw(context.Background(), "")
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestVolumeInspect(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/volumes/volume_id"
	expected := volume.Volume{
		Name:       "name",
		Driver:     "driver",
		Mountpoint: "mountpoint",
	}

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodGet {
				return nil, fmt.Errorf("expected GET method, got %s", req.Method)
			}
			content, err := json.Marshal(expected)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		})),
	)
	assert.NilError(t, err)

	vol, err := client.VolumeInspect(context.Background(), "volume_id")
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(expected, vol))
}
