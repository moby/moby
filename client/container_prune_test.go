package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainersPruneError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ContainersPrune(context.Background(), ContainerPruneOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainersPrune(t *testing.T) {
	const expectedURL = "/containers/prune"

	listCases := []struct {
		filters             Filters
		expectedQueryParams map[string]string
	}{
		{
			filters: Filters{},
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			filters: make(Filters).Add("dangling", "true"),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true}}`,
			},
		},
		{
			filters: make(Filters).
				Add("dangling", "true").
				Add("until", "2016-12-15T14:00"),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"until":{"2016-12-15T14:00":true}}`,
			},
		},
		{
			filters: make(Filters).Add("dangling", "false"),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
		{
			filters: make(Filters).
				Add("dangling", "true").
				Add("label", "label1=foo").
				Add("label", "label2!=bar"),
			expectedQueryParams: map[string]string{
				"until":   "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"label":{"label1=foo":true,"label2!=bar":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			for key, expected := range listCase.expectedQueryParams {
				actual := query.Get(key)
				assert.Check(t, is.Equal(expected, actual))
			}
			content, err := json.Marshal(container.PruneReport{
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
		}))
		assert.NilError(t, err)

		req, err := client.ContainersPrune(context.Background(), ContainerPruneOptions{Filters: listCase.filters})
		assert.NilError(t, err)
		assert.Check(t, is.Len(req.Report.ContainersDeleted, 2))
		assert.Check(t, is.Equal(uint64(9999), req.Report.SpaceReclaimed))
	}
}
