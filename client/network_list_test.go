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
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworkListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.NetworkList(context.Background(), NetworkListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestNetworkList(t *testing.T) {
	const expectedURL = "/networks"

	listCases := []struct {
		options         NetworkListOptions
		expectedFilters string
	}{
		{
			options:         NetworkListOptions{},
			expectedFilters: "",
		},
		{
			options: NetworkListOptions{
				Filters: filters.NewArgs(filters.Arg("dangling", "false")),
			},
			expectedFilters: `{"dangling":{"false":true}}`,
		},
		{
			options: NetworkListOptions{
				Filters: filters.NewArgs(filters.Arg("dangling", "true")),
			},
			expectedFilters: `{"dangling":{"true":true}}`,
		},
		{
			options: NetworkListOptions{
				Filters: filters.NewArgs(
					filters.Arg("label", "label1"),
					filters.Arg("label", "label2"),
				),
			},
			expectedFilters: `{"label":{"label1":true,"label2":true}}`,
		},
	}

	for _, listCase := range listCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				if req.Method != http.MethodGet {
					return nil, fmt.Errorf("expected GET method, got %s", req.Method)
				}
				query := req.URL.Query()
				actualFilters := query.Get("filters")
				if actualFilters != listCase.expectedFilters {
					return nil, fmt.Errorf("filters not set in URL query properly. Expected '%s', got %s", listCase.expectedFilters, actualFilters)
				}
				content, err := json.Marshal([]network.Summary{
					{
						Name:   "network",
						Driver: "bridge",
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

		networkResources, err := client.NetworkList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(networkResources, 1))
	}
}
