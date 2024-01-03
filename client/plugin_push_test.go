package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginPushError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.PluginPush(context.Background(), "plugin_name", "")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestPluginPush(t *testing.T) {
	expectedURL := "/plugins/plugin_name"

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != http.MethodPost {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			auth := req.Header.Get(registry.AuthHeader)
			if auth != "authtoken" {
				return nil, fmt.Errorf("invalid auth header: expected 'authtoken', got %s", auth)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	_, err := client.PluginPush(context.Background(), "plugin_name", "authtoken")
	if err != nil {
		t.Fatal(err)
	}
}
