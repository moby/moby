package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/volume"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestVolumeListError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
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
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			actualFilters := query.Get("filters")
			if actualFilters != listCase.expectedFilters {
				return nil, fmt.Errorf("filters not set in URL query properly. Expected '%s', got %s", listCase.expectedFilters, actualFilters)
			}
			return mockJSONResponse(http.StatusOK, nil, volume.ListResponse{
				Volumes: []*volume.Volume{
					{
						Name:   "volume",
						Driver: "local",
					},
				},
			})(req)
		}))
		assert.NilError(t, err)

		result, err := client.VolumeList(context.Background(), VolumeListOptions{Filters: listCase.filters})
		assert.NilError(t, err)
		assert.Check(t, is.Len(result.Items, 1))
	}
}
