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
)

func TestNetworkListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.NetworkList(context.Background(), types.NetworkListOptions{
		Filters: filters.NewArgs(),
	})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestNetworkList(t *testing.T) {
	expectedURL := "/networks"

	noDanglingFilters := filters.NewArgs()
	noDanglingFilters.Add("dangling", "false")

	danglingFilters := filters.NewArgs()
	danglingFilters.Add("dangling", "true")

	labelFilters := filters.NewArgs()
	labelFilters.Add("label", "label1")
	labelFilters.Add("label", "label2")

	listCases := []struct {
		options         types.NetworkListOptions
		expectedFilters string
	}{
		{
			options: types.NetworkListOptions{
				Filters: filters.NewArgs(),
			},
			expectedFilters: "",
		}, {
			options: types.NetworkListOptions{
				Filters: noDanglingFilters,
			},
			expectedFilters: `{"dangling":{"false":true}}`,
		}, {
			options: types.NetworkListOptions{
				Filters: danglingFilters,
			},
			expectedFilters: `{"dangling":{"true":true}}`,
		}, {
			options: types.NetworkListOptions{
				Filters: labelFilters,
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
				content, err := json.Marshal([]types.NetworkResource{
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
		if err != nil {
			t.Fatal(err)
		}
		if len(networkResources) != 1 {
			t.Fatalf("expected 1 network resource, got %v", networkResources)
		}
	}
}
