package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestPluginPushError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", "")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestPluginPush(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/plugins/plugin_name"

	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			auth := req.Header.Get("X-Registry-Auth")
			if auth != "authtoken" {
				return nil, fmt.Errorf("Invalid auth header : expected 'authtoken', got %s", auth)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		})),
	)
	assert.NilError(t, err)

	_, err = client.PluginPush(context.Background(), "plugin_name", "authtoken")
	if err != nil {
		t.Fatal(err)
	}
}
