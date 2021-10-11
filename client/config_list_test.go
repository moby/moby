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
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestConfigListUnsupported(t *testing.T) {
	client := &Client{
		version: "1.29",
		client:  &http.Client{},
	}
	_, err := client.ConfigList(context.Background(), types.ConfigListOptions{})
	assert.Check(t, is.Error(err, `"config list" requires API version 1.30, but the Docker daemon API version is 1.29`))
}

func TestConfigListError(t *testing.T) {
	client := &Client{
		version: "1.30",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ConfigList(context.Background(), types.ConfigListOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestConfigList(t *testing.T) {
	expectedURL := "/v1.30/configs"

	filters := filters.NewArgs()
	filters.Add("label", "label1")
	filters.Add("label", "label2")

	listCases := []struct {
		options             types.ConfigListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: types.ConfigListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: types.ConfigListOptions{
				Filters: filters,
			},
			expectedQueryParams: map[string]string{
				"filters": `{"label":{"label1":true,"label2":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client := &Client{
			version: "1.30",
			client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
		}

		configs, err := client.ConfigList(context.Background(), listCase.options)
		if err != nil {
			t.Fatal(err)
		}
		if len(configs) != 2 {
			t.Fatalf("expected 2 configs, got %v", configs)
		}
	}
}
