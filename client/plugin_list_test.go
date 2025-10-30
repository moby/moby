package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/plugin"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginListError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.PluginList(context.Background(), PluginListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestPluginList(t *testing.T) {
	const expectedURL = "/plugins"

	listCases := []struct {
		filters             Filters
		expectedQueryParams map[string]string
	}{
		{
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			filters: make(Filters).Add("enabled", "true"),
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"enabled":{"true":true}}`,
			},
		},
		{
			filters: make(Filters).Add("capability", "volumedriver", "authz"),
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"capability":{"authz":true,"volumedriver":true}}`,
			},
		},
	}

	for _, listCase := range listCases {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			for key, expected := range listCase.expectedQueryParams {
				actual := query.Get(key)
				if actual != expected {
					return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
				}
			}
			return mockJSONResponse(http.StatusOK, nil, []*plugin.Plugin{
				{ID: "plugin_id1"},
				{ID: "plugin_id2"},
			})(req)
		}))
		assert.NilError(t, err)

		list, err := client.PluginList(context.Background(), PluginListOptions{
			Filters: listCase.filters,
		})
		assert.NilError(t, err)
		assert.Check(t, is.Len(list.Items, 2))
	}
}
