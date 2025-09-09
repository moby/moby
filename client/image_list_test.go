package client

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

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/image"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageListError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageList(context.Background())
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestImageListConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestImageListConnectionError(t *testing.T) {
	client, err := NewClientWithOpts(WithAPIVersionNegotiation(), WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ImageList(context.Background())
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestImageList(t *testing.T) {
	const expectedURL = "/images/json"

	listCases := []struct {
		options             []ImageListOption
		expectedQueryParams map[string]string
	}{
		{
			options: nil,
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			options: []ImageListOption{
				ImageListWithFilters(filters.NewArgs(
					filters.Arg("label", "label1"),
					filters.Arg("label", "label2"),
					filters.Arg("dangling", "true"),
				)),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"label":{"label1":true,"label2":true}}`,
			},
		},
		{
			options: []ImageListOption{
				ImageListWithFilters(filters.NewArgs(filters.Arg("dangling", "false"))),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
			content, err := json.Marshal([]image.Summary{
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
		}))
		assert.NilError(t, err)

		images, err := client.ImageList(context.Background(), listCase.options...)
		assert.NilError(t, err)
		assert.Check(t, is.Len(images, 2))
	}
}

func TestImageListApiBefore125(t *testing.T) {
	expectedFilter := "image:tag"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		query := req.URL.Query()
		actualFilter := query.Get("filter")
		if actualFilter != expectedFilter {
			return nil, fmt.Errorf("filter not set in URL query properly. Expected '%s', got %s", expectedFilter, actualFilter)
		}
		actualFilters := query.Get("filters")
		if actualFilters != "" {
			return nil, fmt.Errorf("filters should have not been present, were with value: %s", actualFilters)
		}
		content, err := json.Marshal([]image.Summary{
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
	}), WithVersion("1.24"))
	assert.NilError(t, err)

	images, err := client.ImageList(context.Background(),
		ImageListWithFilters(filters.NewArgs(filters.Arg("reference", "image:tag"))))
	assert.NilError(t, err)
	assert.Check(t, is.Len(images, 2))
}

// Checks if shared-size query parameter is set/not being set correctly
// for /images/json.
func TestImageListWithSharedSize(t *testing.T) {
	t.Parallel()
	const sharedSize = "shared-size"
	for _, tc := range []struct {
		name       string
		version    string
		options    []ImageListOption
		sharedSize string // expected value for the shared-size query param, or empty if it should not be set.
	}{
		{name: "unset after 1.42, no options set", version: "1.42"},
		{name: "set after 1.42, if requested", version: "1.42", options: []ImageListOption{ImageListWithSharedSize(true)}, sharedSize: "1"},
		{name: "unset before 1.42, even if requested", version: "1.41", options: []ImageListOption{ImageListWithSharedSize(true)}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var query url.Values
			client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
				query = req.URL.Query()
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader("[]")),
				}, nil
			}), WithVersion(tc.version))
			assert.NilError(t, err)
			_, err = client.ImageList(context.Background(), tc.options...)
			assert.NilError(t, err)
			expectedSet := tc.sharedSize != ""
			assert.Check(t, is.Equal(query.Has(sharedSize), expectedSet))
			assert.Check(t, is.Equal(query.Get(sharedSize), tc.sharedSize))
		})
	}
}
