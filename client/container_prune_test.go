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
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainersPruneError(t *testing.T) {
	client := &Client{
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
		version: "1.25",
	}

	filters := filters.NewArgs()

	_, err := client.ContainersPrune(context.Background(), filters)
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainersPrune(t *testing.T) {
	expectedURL := "/v1.25/containers/prune"

	danglingFilters := filters.NewArgs()
	danglingFilters.Add("dangling", "true")

	noDanglingFilters := filters.NewArgs()
	noDanglingFilters.Add("dangling", "false")

	danglingUntilFilters := filters.NewArgs()
	danglingUntilFilters.Add("dangling", "true")
	danglingUntilFilters.Add("until", "2016-12-15T14:00")

	labelFilters := filters.NewArgs()
	labelFilters.Add("dangling", "true")
	labelFilters.Add("label", "label1=foo")
	labelFilters.Add("label", "label2!=bar")

	listCases := []struct {
		filters             filters.Args
		expectedQueryParams map[string]string
	}{
		{
			filters: filters.Args{},
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			filters: danglingFilters,
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true}}`,
			},
		},
		{
			filters: danglingUntilFilters,
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"until":{"2016-12-15T14:00":true}}`,
			},
		},
		{
			filters: noDanglingFilters,
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
		{
			filters: labelFilters,
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"label":{"label1=foo":true,"label2!=bar":true}}`,
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
					assert.Check(t, is.Equal(expected, actual))
				}
				content, err := json.Marshal(types.ContainersPruneReport{
					ContainersDeleted: []string{"container_id1", "container_id2"},
					SpaceReclaimed:    9999,
				})
				if err != nil {
					return nil, err
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader(content)),
				}, nil
			}),
			version: "1.25",
		}

		report, err := client.ContainersPrune(context.Background(), listCase.filters)
		assert.Check(t, err)
		assert.Check(t, is.Len(report.ContainersDeleted, 2))
		assert.Check(t, is.Equal(uint64(9999), report.SpaceReclaimed))
	}
}
