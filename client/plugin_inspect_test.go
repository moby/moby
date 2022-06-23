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
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

func TestPluginInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, _, err = client.PluginInspectWithRaw(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestPluginInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, _, err = client.PluginInspectWithRaw(context.Background(), "")
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestPluginInspect(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/plugins/plugin_name"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(types.Plugin{
				ID: "plugin_id",
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

	pluginInspect, _, err := client.PluginInspectWithRaw(context.Background(), "plugin_name")
	if err != nil {
		t.Fatal(err)
	}
	if pluginInspect.ID != "plugin_id" {
		t.Fatalf("expected `plugin_id`, got %s", pluginInspect.ID)
	}
}
