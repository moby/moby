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
	"github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.VolumeList(context.Background(), VolumeListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestVolumeList(t *testing.T) {
	const expectedURL = "/volumes"

	listCases := []struct {
		filters         filters.Args
		expectedFilters string
	}{
		{
			filters:         filters.NewArgs(),
			expectedFilters: "",
		}, {
			filters:         filters.NewArgs(filters.Arg("dangling", "false")),
			expectedFilters: `{"dangling":{"false":true}}`,
		}, {
			filters:         filters.NewArgs(filters.Arg("dangling", "true")),
			expectedFilters: `{"dangling":{"true":true}}`,
		}, {
			filters: filters.NewArgs(
				filters.Arg("label", "label1"),
				filters.Arg("label", "label2"),
			),
			expectedFilters: `{"label":{"label1":true,"label2":true}}`,
		},
	}

	for _, listCase := range listCases {
		client := &Client{
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				actualFilters := query.Get("filters")
				if actualFilters != listCase.expectedFilters {
					return nil, fmt.Errorf("filters not set in URL query properly. Expected '%s', got %s", listCase.expectedFilters, actualFilters)
				}
				content, err := json.Marshal(volume.ListResponse{
					Volumes: []*volume.Volume{
						{
							Name:   "volume",
							Driver: "local",
						},
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

		volumeResponse, err := client.VolumeList(context.Background(), VolumeListOptions{Filters: listCase.filters})
		assert.NilError(t, err)
		assert.Check(t, is.Len(volumeResponse.Volumes, 1))
	}
}
