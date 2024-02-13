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

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageRemoveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ImageRemove(context.Background(), "image_id", image.RemoveOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageRemoveImageNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "no such image: unknown")),
	}

	_, err := client.ImageRemove(context.Background(), "unknown", image.RemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "no such image: unknown"))
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestImageRemove(t *testing.T) {
	expectedURL := "/images/image_id"
	removeCases := []struct {
		force               bool
		pruneChildren       bool
		expectedQueryParams map[string]string
	}{
		{
			force:         false,
			pruneChildren: false,
			expectedQueryParams: map[string]string{
				"force":   "",
				"noprune": "1",
			},
		}, {
			force:         true,
			pruneChildren: true,
			expectedQueryParams: map[string]string{
				"force":   "1",
				"noprune": "",
			},
		},
	}
	for _, removeCase := range removeCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				if req.Method != http.MethodDelete {
					return nil, fmt.Errorf("expected DELETE method, got %s", req.Method)
				}
				query := req.URL.Query()
				for key, expected := range removeCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				b, err := json.Marshal([]image.DeleteResponse{
					{
						Untagged: "image_id1",
					},
					{
						Deleted: "image_id",
					},
				})
				if err != nil {
					return nil, err
				}

				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(b)),
				}, nil
			}),
		}
		imageDeletes, err := client.ImageRemove(context.Background(), "image_id", image.RemoveOptions{
			Force:         removeCase.force,
			PruneChildren: removeCase.pruneChildren,
		})
		if err != nil {
			t.Fatal(err)
		}
		if len(imageDeletes) != 2 {
			t.Fatalf("expected 2 deleted images, got %v", imageDeletes)
		}
	}
}
