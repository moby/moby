package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageLoadError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageLoad(context.Background(), nil, ImageLoadWithQuiet(true))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestImageLoad(t *testing.T) {
	const (
		expectedURL         = "/images/load"
		expectedContentType = "application/x-tar"
		expectedInput       = "inputBody"
		expectedOutput      = `{"stream":"Loaded image: busybox:latest\n"}`
	)
	tests := []struct {
		doc                 string
		quiet               bool
		platforms           []ocispec.Platform
		expectedQueryParams url.Values
	}{
		{
			doc: "no options",
			expectedQueryParams: url.Values{
				"quiet": {"0"},
			},
		},
		{
			doc:   "quiet",
			quiet: true,
			expectedQueryParams: url.Values{
				"quiet": {"1"},
			},
		},
		{
			doc:       "with platform",
			platforms: []ocispec.Platform{{Architecture: "arm64", OS: "linux", Variant: "v8"}},
			expectedQueryParams: url.Values{
				"platform": {`{"architecture":"arm64","os":"linux","variant":"v8"}`},
				"quiet":    {"0"},
			},
		},
		{
			doc: "multiple platforms",
			platforms: []ocispec.Platform{
				{Architecture: "arm64", OS: "linux", Variant: "v8"},
				{Architecture: "amd64", OS: "linux"},
			},
			expectedQueryParams: url.Values{
				"platform": {`{"architecture":"arm64","os":"linux","variant":"v8"}`, `{"architecture":"amd64","os":"linux"}`},
				"quiet":    {"0"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
				assert.Check(t, assertRequest(req, http.MethodPost, expectedURL))
				assert.Check(t, is.Equal(req.Header.Get("Content-Type"), expectedContentType))
				assert.Check(t, is.DeepEqual(req.URL.Query(), tc.expectedQueryParams))

				return mockJSONResponse(http.StatusOK, nil, json.RawMessage(expectedOutput))(req)
			}))
			assert.NilError(t, err)

			input := bytes.NewReader([]byte(expectedInput))
			imageLoadResponse, err := client.ImageLoad(context.Background(), input,
				ImageLoadWithQuiet(tc.quiet),
				ImageLoadWithPlatforms(tc.platforms...),
			)
			assert.NilError(t, err)

			body, err := io.ReadAll(imageLoadResponse)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(string(body), expectedOutput))
		})
	}
}
