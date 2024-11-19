package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageImportError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ImageImport(context.Background(), image.ImportSource{}, "image:tag", image.ImportOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestImageImport(t *testing.T) {
	const (
		expectedURL    = "/images/create"
		expectedOutput = "outputBody"
	)
	tests := []struct {
		doc                 string
		options             image.ImportOptions
		expectedQueryParams url.Values
	}{
		{
			doc: "no options",
			expectedQueryParams: url.Values{
				"fromSrc": {"image_source"},
				"message": {""},
				"repo":    {"repository_name:imported"},
				"tag":     {""},
			},
		},
		{
			doc: "change options",
			options: image.ImportOptions{
				Tag:     "imported",
				Message: "A message",
				Changes: []string{"change1", "change2"},
			},
			expectedQueryParams: url.Values{
				"changes": {"change1", "change2"},
				"fromSrc": {"image_source"},
				"message": {"A message"},
				"repo":    {"repository_name:imported"},
				"tag":     {"imported"},
			},
		},
		{
			doc: "with platform",
			options: image.ImportOptions{
				Platform: "linux/amd64",
			},
			expectedQueryParams: url.Values{
				"fromSrc":  {"image_source"},
				"message":  {""},
				"platform": {"linux/amd64"},
				"repo":     {"repository_name:imported"},
				"tag":      {""},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					assert.Check(t, is.Equal(req.URL.Path, expectedURL))
					query := req.URL.Query()
					assert.Check(t, is.DeepEqual(query, tc.expectedQueryParams))
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(expectedOutput))),
					}, nil
				}),
			}
			resp, err := client.ImageImport(context.Background(), image.ImportSource{
				Source:     strings.NewReader("source"),
				SourceName: "image_source",
			}, "repository_name:imported", tc.options)
			assert.NilError(t, err)
			defer assert.NilError(t, resp.Close())

			body, err := io.ReadAll(resp)
			assert.NilError(t, err)
			assert.Equal(t, string(body), expectedOutput)
		})
	}
}
