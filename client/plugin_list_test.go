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
	"github.com/moby/moby/api/types/plugin"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestPluginListError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.PluginList(context.Background(), nil)
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
		client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
			content, err := json.Marshal([]*plugin.Plugin{
				{
					ID: "plugin_id1",
				},
				{
					ID: "plugin_id2",
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

		plugins, err := client.PluginList(context.Background(), listCase.filters)
		assert.NilError(t, err)
		assert.Check(t, is.Len(plugins, 2))
	}
}
