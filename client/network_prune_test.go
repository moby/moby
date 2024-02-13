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

func TestNetworksPruneError(t *testing.T) {
	client := &Client{
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
		version: "1.25",
	}

	_, err := client.NetworksPrune(context.Background(), filters.NewArgs())
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestNetworksPrune(t *testing.T) {
	const expectedURL = "/v1.25/networks/prune"

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
			filters: filters.NewArgs(filters.Arg("dangling", "true")),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true}}`,
			},
		},
		{
			filters: filters.NewArgs(filters.Arg("dangling", "false")),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
		{
			filters: filters.NewArgs(
				filters.Arg("dangling", "true"),
				filters.Arg("label", "label1=foo"),
				filters.Arg("label", "label2!=bar"),
			),
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
				content, err := json.Marshal(types.NetworksPruneReport{
					NetworksDeleted: []string{"network_id1", "network_id2"},
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

		report, err := client.NetworksPrune(context.Background(), listCase.filters)
		assert.Check(t, err)
		assert.Check(t, is.Len(report.NetworksDeleted, 2))
	}
}
