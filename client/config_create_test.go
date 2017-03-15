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
	"github.com/docker/docker/api/types/swarm"
	"golang.org/x/net/context"
)

func TestConfigCreateError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	if err == nil || err.Error() != "Error response from daemon: Server error" {
		t.Fatalf("expected a Server Error, got %v", err)
	}
}

func TestConfigCreate(t *testing.T) {
	expectedURL := "/configs/create"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			if req.Method != "POST" {
				return nil, fmt.Errorf("expected POST method, got %s", req.Method)
			}
			b, err := json.Marshal(types.ConfigCreateResponse{
				ID: "test_config",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       ioutil.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	r, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "test_config" {
		t.Fatalf("expected `test_config`, got %s", r.ID)
	}
}
