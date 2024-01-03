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
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestDiskUsageError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.DiskUsage(context.Background(), types.DiskUsageOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestDiskUsage(t *testing.T) {
	expectedURL := "/system/df"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}

			du := types.DiskUsage{
				LayersSize: int64(100),
				Images:     nil,
				Containers: nil,
				Volumes:    nil,
			}

			b, err := json.Marshal(du)
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}
	if _, err := client.DiskUsage(context.Background(), types.DiskUsageOptions{}); err != nil {
		t.Fatal(err)
	}
}
