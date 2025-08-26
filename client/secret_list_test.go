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

func TestSecretListUnsupported(t *testing.T) {
	client := &Client{
		version: "1.24",
		client:  &http.Client{},
	}
	_, err := client.SecretList(context.Background(), SecretListOptions{})
	assert.Check(t, is.Error(err, `"secret list" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretListError(t *testing.T) {
	client := &Client{
		version: "1.25",
		client:  newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.SecretList(context.Background(), SecretListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretList(t *testing.T) {
	const expectedURL = "/v1.25/secrets"

	listCases := []struct {
		options             SecretListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: SecretListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: SecretListOptions{
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
		client := &Client{
			version: "1.25",
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
				content, err := json.Marshal([]swarm.Secret{
					{
						ID: "secret_id1",
					},
					{
						ID: "secret_id2",
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

		secrets, err := client.SecretList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(secrets, 2))
	}
}
