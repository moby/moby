package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageLoadError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ImageLoad(context.Background(), nil, image.LoadOptions{Quiet: true})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageLoad(t *testing.T) {
	const (
		expectedURL         = "/images/load"
		expectedContentType = "application/x-tar"
		expectedInput       = "inputBody"
		expectedOutput      = "outputBody"
	)
	tests := []struct {
		doc                  string
		quiet                bool
		platform             *ocispec.Platform
		responseContentType  string
		expectedResponseJSON bool
		expectedQueryParams  url.Values
	}{
		{
			doc:                  "plain-text",
			quiet:                false,
			responseContentType:  "text/plain",
			expectedResponseJSON: false,
			expectedQueryParams: url.Values{
				"quiet": {"0"},
			},
		},
		{
			doc:                  "json quiet",
			quiet:                true,
			responseContentType:  "application/json",
			expectedResponseJSON: true,
			expectedQueryParams: url.Values{
				"quiet": {"1"},
			},
		},
		{
			doc:                  "json with platform",
			platform:             &ocispec.Platform{Architecture: "arm64", OS: "linux", Variant: "v8"},
			responseContentType:  "application/json",
			expectedResponseJSON: true,
			expectedQueryParams: url.Values{
				"platform": {`{"architecture":"arm64","os":"linux","variant":"v8"}`},
				"quiet":    {"0"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					assert.Check(t, is.Equal(req.URL.Path, expectedURL))
					assert.Check(t, is.Equal(req.Header.Get("Content-Type"), expectedContentType))
					assert.Check(t, is.DeepEqual(req.URL.Query(), tc.expectedQueryParams))
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
						Header:     http.Header{"Content-Type": []string{tc.responseContentType}},
					}, nil
				}),
			}

			input := bytes.NewReader([]byte(expectedInput))
			imageLoadResponse, err := client.ImageLoad(context.Background(), input, image.LoadOptions{
				Quiet:    tc.quiet,
				Platform: tc.platform,
			})
			assert.NilError(t, err)
			assert.Check(t, is.Equal(imageLoadResponse.JSON, tc.expectedResponseJSON))

			body, err := io.ReadAll(imageLoadResponse.Body)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(body), expectedOutput))
		})
	}
}
