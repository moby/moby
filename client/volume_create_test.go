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
	"gotest.tools/v3/assert"
)

func TestVolumeCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.VolumeCreate(context.Background(), volume.CreateOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestVolumeCreate(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/volumes/create"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			content, err := json.Marshal(volume.Volume{
				Name:       "volume",
				Driver:     "local",
				Mountpoint: "mountpoint",
			})
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

	vol, err := client.VolumeCreate(context.Background(), volume.CreateOptions{
		Name:   "myvolume",
		Driver: "mydriver",
		DriverOpts: map[string]string{
			"opt-key": "opt-value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if vol.Name != "volume" {
		t.Fatalf("expected volume.Name to be 'volume', got %s", vol.Name)
	}
	if vol.Driver != "local" {
		t.Fatalf("expected volume.Driver to be 'local', got %s", vol.Driver)
	}
	if vol.Mountpoint != "mountpoint" {
		t.Fatalf("expected volume.Mountpoint to be 'mountpoint', got %s", vol.Mountpoint)
	}
}
