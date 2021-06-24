package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
)

func TestBuildDiskUsageError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.BuildDiskUsage(context.Background())
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestBuildDiskUsage(t *testing.T) {
	expectedURL := "/builds/usage"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			du := []*types.BuildCache{
				{
					ID:          "test-id",
					Parent:      "test-parent",
					Type:        "test-type",
					Description: "test-description",
					Size:        42,
					CreatedAt:   time.Now(),
				},
			}

			b, err := json.Marshal(du)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}
	if _, err := client.BuildDiskUsage(context.Background()); err != nil {
		t.Fatal(err)
	}
}
