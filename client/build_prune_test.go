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
	"github.com/moby/moby/api/types/build"
	"github.com/moby/moby/api/types/filters"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestBuildCachePruneError(t *testing.T) {
	client := &Client{
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
		version: "1.31",
	}

	_, err := client.BuildCachePrune(context.Background(), build.CachePruneOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestBuildCachePrune(t *testing.T) {
	expectedURL := "/v1.31/build/prune"

	testCases := []struct {
		options             build.CachePruneOptions
		expectedQueryParams map[string]string
	}{
		{
			options: build.CachePruneOptions{},
			expectedQueryParams: map[string]string{
				"all":            "",
				"filters":        "",
				"reserved-space": "",
				"max-used-space": "",
				"min-free-space": "",
			},
		},
		{
			options: build.CachePruneOptions{
				All: true,
			},
			expectedQueryParams: map[string]string{
				"all":            "1",
				"filters":        "",
				"reserved-space": "",
				"max-used-space": "",
				"min-free-space": "",
			},
		},
		{
			options: build.CachePruneOptions{
				ReservedSpace: 1024,
				MaxUsedSpace:  2048,
				MinFreeSpace:  512,
			},
			expectedQueryParams: map[string]string{
				"all":            "",
				"filters":        "",
				"reserved-space": "1024",
				"max-used-space": "2048",
				"min-free-space": "512",
			},
		},
		{
			options: build.CachePruneOptions{
				Filters: filters.NewArgs(filters.Arg("unused-for", "24h")),
			},
			expectedQueryParams: map[string]string{
				"all":            "",
				"filters":        `{"unused-for":{"24h":true}}`,
				"reserved-space": "",
				"max-used-space": "",
				"min-free-space": "",
			},
		},
	}

	for _, testCase := range testCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				for key, expected := range testCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("Expected '%s' for '%s', got '%s'", expected, key, actual)
					}
				}
				// Ensure keep-storage parameter is NOT present
				if query.Get("keep-storage") != "" {
					return nil, fmt.Errorf("keep-storage parameter should not be present, but found: %s", query.Get("keep-storage"))
				}
				report := build.CachePruneReport{
					CachesDeleted:  []string{"cache1", "cache2"},
					SpaceReclaimed: 12345,
				}
				b, err := json.Marshal(report)
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(b)),
				}, nil
			}),
			version: "1.31",
		}

		_, err := client.BuildCachePrune(context.Background(), testCase.options)
		assert.NilError(t, err)
	}
}