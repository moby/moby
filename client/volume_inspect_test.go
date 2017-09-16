package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"
)

func TestVolumeInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.VolumeInspect(context.Background(), "nothing")
	testutil.ErrorContains(t, err, "Error response from daemon: Server error")
}

func TestVolumeInspectNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}

	_, err := client.VolumeInspect(context.Background(), "unknown")
	assert.True(t, IsErrNotFound(err))
}

func TestVolumeInspectWithEmptyID(t *testing.T) {
	expectedURL := "/volumes/"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, req.URL.Path, expectedURL)
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       ioutil.NopCloser(bytes.NewReader(nil)),
			}, nil
		}),
	}
	_, err := client.VolumeInspect(context.Background(), "")
	testutil.ErrorContains(t, err, "No such volume: ")

}

func TestVolumeInspect(t *testing.T) {
	expectedURL := "/volumes/volume_id"
	expected := types.Volume{
		Name:       "name",
		Driver:     "driver",
		Mountpoint: "mountpoint",
	}

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "GET" {
				return nil, fmt.Errorf("expected GET method, got %s", req.Method)
			}
			content, err := json.Marshal(expected)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	volume, err := client.VolumeInspect(context.Background(), "volume_id")
	require.NoError(t, err)
	assert.Equal(t, expected, volume)
}
