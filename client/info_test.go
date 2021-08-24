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
	"github.com/docker/docker/errdefs"
)

func TestInfoServerError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.Info(context.Background())
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
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
	if err == nil || !strings.Contains(err.Error(), "invalid character") {
		t.Fatalf("expected a 'invalid character' error, got %v", err)
	}
}

func TestInfo(t *testing.T) {
	expectedURL := "/info"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			info := &types.Info{
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
	if err != nil {
		t.Fatal(err)
	}

	if info.ID != "daemonID" {
		t.Fatalf("expected daemonID, got %s", info.ID)
	}

	if info.Containers != 3 {
		t.Fatalf("expected 3 containers, got %d", info.Containers)
	}
}
