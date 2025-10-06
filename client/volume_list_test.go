package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeListError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.VolumeList(context.Background(), VolumeListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestVolumeList(t *testing.T) {
	const expectedURL = "/volumes"

	listCases := []struct {
		filters         Filters
		expectedFilters string
	}{
		{
			expectedFilters: "",
		}, {
			filters:         make(Filters).Add("dangling", "false"),
			expectedFilters: `{"dangling":{"false":true}}`,
		}, {
			filters:         make(Filters).Add("dangling", "true"),
			expectedFilters: `{"dangling":{"true":true}}`,
		}, {
			filters:         make(Filters).Add("label", "label1", "label2"),
			expectedFilters: `{"label":{"label1":true,"label2":true}}`,
		},
	}

	for _, listCase := range listCases {
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
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
		}))
		assert.NilError(t, err)

		volumeResponse, err := client.VolumeList(context.Background(), VolumeListOptions{Filters: listCase.filters})
		assert.NilError(t, err)
		assert.Check(t, is.Len(volumeResponse.Volumes, 1))
	}
}
