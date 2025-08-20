package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageRemoveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ImageRemove(context.Background(), "image_id", ImageRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageRemoveImageNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "no such image: unknown")),
	}

	_, err := client.ImageRemove(context.Background(), "unknown", ImageRemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "no such image: unknown"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestImageRemove(t *testing.T) {
	expectedURL := "/images/image_id"
	removeCases := []struct {
		force               bool
		pruneChildren       bool
		platform            *ocispec.Platform
		expectedQueryParams map[string]string
	}{
		{
			force:         false,
			pruneChildren: false,
			expectedQueryParams: map[string]string{
				"force":   "",
				"noprune": "1",
			},
		},
		{
			force:         true,
			pruneChildren: true,
			expectedQueryParams: map[string]string{
				"force":   "1",
				"noprune": "",
			},
		},
		{
			platform: &ocispec.Platform{
				Architecture: "amd64",
				OS:           "linux",
			},
			expectedQueryParams: map[string]string{
				"platforms": `{"architecture":"amd64","os":"linux"}`,
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

		opts := ImageRemoveOptions{
			Force:         removeCase.force,
			PruneChildren: removeCase.pruneChildren,
		}
		if removeCase.platform != nil {
			opts.Platforms = []ocispec.Platform{*removeCase.platform}
		}

		imageDeletes, err := client.ImageRemove(context.Background(), "image_id", opts)
		assert.NilError(t, err)
		assert.Check(t, is.Len(imageDeletes, 2))
	}
}
