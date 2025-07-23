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
	"github.com/moby/moby/api/types/system"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestInfoServerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.Info(context.Background())
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestInfoInvalidResponseJSONError(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("invalid json"))),
			}, nil
		}),
	}
	_, err := client.Info(context.Background())
	assert.Check(t, is.ErrorContains(err, "invalid character"))
}

func TestInfo(t *testing.T) {
	expectedURL := "/info"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			info := &system.Info{
				ID:         "daemonID",
				Containers: 3,
			}
			b, err := json.Marshal(info)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	assert.Check(t, is.Equal(info.ID, "daemonID"))
	assert.Check(t, is.Equal(info.Containers, 3))
}

func TestInfoWithDiscoveredDevices(t *testing.T) {
	expectedURL := "/info"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			info := &system.Info{
				ID:         "daemonID",
				Containers: 3,
				DiscoveredDevices: []system.DeviceInfo{
					{
						Source: "cdi",
						ID:     "vendor.com/gpu=0",
					},
					{
						Source: "cdi",
						ID:     "vendor.com/gpu=1",
					},
				},
			}
			b, err := json.Marshal(info)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	info, err := client.Info(context.Background())
	assert.NilError(t, err)

	assert.Check(t, is.Equal(info.ID, "daemonID"))
	assert.Check(t, is.Equal(info.Containers, 3))

	assert.Check(t, is.Len(info.DiscoveredDevices, 2))

	device0 := info.DiscoveredDevices[0]
	assert.Check(t, is.Equal(device0.Source, "cdi"))
	assert.Check(t, is.Equal(device0.ID, "vendor.com/gpu=0"))

	device1 := info.DiscoveredDevices[1]
	assert.Check(t, is.Equal(device1.Source, "cdi"))
	assert.Check(t, is.Equal(device1.ID, "vendor.com/gpu=1"))
}
