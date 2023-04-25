package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ImageList(context.Background(), types.ImageListOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestImageList(t *testing.T) {
	const expectedURL = "/images/json"

	listCases := []struct {
		options             types.ImageListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: types.ImageListOptions{},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			options: types.ImageListOptions{
				Filters: filters.NewArgs(
					filters.Arg("label", "label1"),
					filters.Arg("label", "label2"),
					filters.Arg("dangling", "true"),
				),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"label":{"label1":true,"label2":true}}`,
			},
		},
		{
			options: types.ImageListOptions{
				Filters: filters.NewArgs(filters.Arg("dangling", "false")),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				for key, expected := range listCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				content, err := json.Marshal([]types.ImageSummary{
					{
						ID: "image_id2",
					},
					{
						ID: "image_id2",
					},
				})
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(content)),
				}, nil
			}),
		}

		images, err := client.ImageList(context.Background(), listCase.options)
		if err != nil {
			t.Fatal(err)
		}
		if len(images) != 2 {
			t.Fatalf("expected 2 images, got %v", images)
		}
	}
}

func TestImageListApiBefore125(t *testing.T) {
	expectedFilter := "image:tag"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			query := req.URL.Query()
			actualFilter := query.Get("filter")
			if actualFilter != expectedFilter {
				return nil, fmt.Errorf("filter not set in URL query properly. Expected '%s', got %s", expectedFilter, actualFilter)
			}
			actualFilters := query.Get("filters")
			if actualFilters != "" {
				return nil, fmt.Errorf("filters should have not been present, were with value: %s", actualFilters)
			}
			content, err := json.Marshal([]types.ImageSummary{
				{
					ID: "image_id2",
				},
				{
					ID: "image_id2",
				},
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(content)),
			}, nil
		}),
		version: "1.24",
	}

	options := types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", "image:tag")),
	}

	images, err := client.ImageList(context.Background(), options)
	if err != nil {
		t.Fatal(err)
	}
	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %v", images)
	}
}

// Checks if shared-size query parameter is set/not being set correctly
// for /images/json.
func TestImageListWithSharedSize(t *testing.T) {
	t.Parallel()
	const sharedSize = "shared-size"
	for _, tc := range []struct {
		name       string
		version    string
		options    types.ImageListOptions
		sharedSize string // expected value for the shared-size query param, or empty if it should not be set.
	}{
		{name: "unset after 1.42, no options set", version: "1.42"},
		{name: "set after 1.42, if requested", version: "1.42", options: types.ImageListOptions{SharedSize: true}, sharedSize: "1"},
		{name: "unset before 1.42, even if requested", version: "1.41", options: types.ImageListOptions{SharedSize: true}},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var query url.Values
			client := &Client{
				client: newMockClient(func(req *http.Request) (*http.Response, error) {
					query = req.URL.Query()
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(strings.NewReader("[]")),
					}, nil
				}),
				version: tc.version,
			}
			_, err := client.ImageList(context.Background(), tc.options)
			assert.Check(t, err)
			expectedSet := tc.sharedSize != ""
			assert.Check(t, is.Equal(query.Has(sharedSize), expectedSet))
			assert.Check(t, is.Equal(query.Get(sharedSize), tc.sharedSize))
		})
	}
}
