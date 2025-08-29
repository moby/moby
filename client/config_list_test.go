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
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestConfigListUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.29"),
		WithHTTPClient(&http.Client{}),
	)
	assert.NilError(t, err)

	_, err = client.ConfigList(context.Background(), ConfigListOptions{})
	assert.Check(t, is.Error(err, `"config list" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestConfigListError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.30"),
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigList(context.Background(), ConfigListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestConfigList(t *testing.T) {
	expectedURL := "/v1.30/configs"

	listCases := []struct {
		options             ConfigListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: ConfigListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: ConfigListOptions{
				Filters: filters.NewArgs(
					filters.Arg("label", "label1"),
					filters.Arg("label", "label2"),
				),
			},
			expectedQueryParams: map[string]string{
				"filters": `{"label":{"label1":true,"label2":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client, err := NewClientWithOpts(
			WithVersion("1.30"),
			WithMockClient(func(req *http.Request) (*http.Response, error) {
				if !strings.HasPrefix(req.URL.Path, expectedURL) {
					return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
				}
				query := req.URL.Query()
				for key, expected := range listCase.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				content, err := json.Marshal([]swarm.Config{
					{
						ID: "config_id1",
					},
					{
						ID: "config_id2",
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
		)
		assert.NilError(t, err)

		configs, err := client.ConfigList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(configs, 2))
	}
}
