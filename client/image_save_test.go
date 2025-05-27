package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageSaveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	armv64 := ocispec.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"}
	_, err := client.ImageSave(context.Background(), []string{"nothing"}, ImageSaveWithPlatforms(armv64))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageSave(t *testing.T) {
	const (
		expectedURL    = "/images/get"
		expectedOutput = "outputBody"
	)
	tests := []struct {
		doc                 string
		options             []ImageSaveOption
		expectedQueryParams url.Values
	}{
		{
			doc: "no platform",
			expectedQueryParams: url.Values{
				"names": {"image_id1", "image_id2"},
			},
		},
		{
			doc: "platform",
			options: []ImageSaveOption{
				ImageSaveWithPlatforms(ocispec.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"}),
			},
			expectedQueryParams: url.Values{
				"names":    {"image_id1", "image_id2"},
				"platform": {`{"architecture":"arm64","os":"linux","variant":"v8"}`},
			},
		},
		{
			doc: "multiple platforms",
			options: []ImageSaveOption{
				ImageSaveWithPlatforms(
					ocispec.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"},
					ocispec.Platform{Architecture: "amd64", OS: "linux"},
				),
			},
			expectedQueryParams: url.Values{
				"names":    {"image_id1", "image_id2"},
				"platform": {`{"architecture":"arm64","os":"linux","variant":"v8"}`, `{"architecture":"amd64","os":"linux"}`},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					assert.Check(t, is.Equal(req.URL.Path, expectedURL))
					assert.Check(t, is.DeepEqual(req.URL.Query(), tc.expectedQueryParams))
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
					}, nil
				}),
			}
			resp, err := client.ImageSave(context.Background(), []string{"image_id1", "image_id2"}, tc.options...)
			assert.NilError(t, err)
			defer assert.NilError(t, resp.Close())

			body, err := io.ReadAll(resp)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(body), expectedOutput))
		})
	}
}
