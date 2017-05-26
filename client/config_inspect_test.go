package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/swarm"
	"golang.org/x/net/context"
)

func TestConfigInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, _, err := client.ConfigInspectWithRaw(context.Background(), "nothing")
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestConfigInspectConfigNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}

	_, _, err := client.ConfigInspectWithRaw(context.Background(), "unknown")
	if err == nil || !IsErrConfigNotFound(err) {
		t.Fatalf("expected a configNotFoundError error, got %v", err)
	}
}

func TestConfigInspect(t *testing.T) {
	expectedURL := "/configs/config_id"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(swarm.Config{
				ID: "config_id",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
	}

	configInspect, _, err := client.ConfigInspectWithRaw(context.Background(), "config_id")
	if err != nil {
		t.Fatal(err)
	}
	if configInspect.ID != "config_id" {
		t.Fatalf("expected `config_id`, got %s", configInspect.ID)
	}
}
