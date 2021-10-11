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

	"github.com/docker/docker/api/types"
	volumetypes "github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/errdefs"
)

func TestVolumeCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestVolumeCreate(t *testing.T) {
	expectedURL := "/volumes/create"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}

			content, err := json.Marshal(types.Volume{
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
		}),
	}

	volume, err := client.VolumeCreate(context.Background(), volumetypes.VolumeCreateBody{
		Name:   "myvolume",
		Driver: "mydriver",
		DriverOpts: map[string]string{
			"opt-key": "opt-value",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if volume.Name != "volume" {
		t.Fatalf("expected volume.Name to be 'volume', got %s", volume.Name)
	}
	if volume.Driver != "local" {
		t.Fatalf("expected volume.Driver to be 'local', got %s", volume.Driver)
	}
	if volume.Mountpoint != "mountpoint" {
		t.Fatalf("expected volume.Mountpoint to be 'mountpoint', got %s", volume.Mountpoint)
	}
}
