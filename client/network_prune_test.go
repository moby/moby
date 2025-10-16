package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNetworksPruneError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.NetworksPrune(context.Background(), NetworkPruneOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestNetworksPrune(t *testing.T) {
	const expectedURL = "/networks/prune"

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
		client, err := NewClientWithOpts(
			WithMockClient(func(req *http.Request) (*http.Response, error) {
				if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
					return nil, err
				}
				query := req.URL.Query()
				for key, expected := range listCase.expectedQueryParams {
					actual := query.Get(key)
					assert.Check(t, is.Equal(expected, actual))
				}
				content, err := json.Marshal(network.PruneReport{
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
		)
		assert.NilError(t, err)

		res, err := client.NetworksPrune(context.Background(), NetworkPruneOptions{Filters: listCase.filters})
		assert.NilError(t, err)
		assert.Check(t, is.Len(res.Report.NetworksDeleted, 2))
	}
}
