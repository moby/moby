// +build experimental

package client

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/net/context"
)

func TestPluginPushError(t *testing.T) {
	client := &Client{
		transport: newMockClient(nil, errorMock(http.StatusInternalServerError, "Server error")),
	}

	err := client.PluginPush(context.Background(), "plugin_name", "")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestPluginPush(t *testing.T) {
	expectedURL := "/plugins/plugin_name"

	client := &Client{
		transport: newMockClient(nil, func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "POST" {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			auth := req.Header.Get("X-Registry-Auth")
			if auth != "authtoken" {
				return nil, fmt.Errorf("Invalid auth header : expected %s, got %s", "authtoken", auth)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.PluginPush(context.Background(), "plugin_name", "authtoken")
	if err != nil {
		t.Fatal(err)
	}
}
