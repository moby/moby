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

func TestSecretListUnsupported(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.24"),
	)
	assert.NilError(t, err)
	_, err = client.SecretList(context.Background(), types.SecretListOptions{})
	assert.Check(t, is.Error(err, `"secret list" requires API version 1.25, but the Docker daemon API version is 1.24`))
}

func TestSecretListError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithVersion("1.25"),
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.SecretList(context.Background(), types.SecretListOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestSecretList(t *testing.T) {
	expectedURL := "/v1.25/secrets"

	filters := filters.NewArgs()
	filters.Add("label", "label1")
	filters.Add("label", "label2")

	listCases := []struct {
		options             types.SecretListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: types.SecretListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: types.SecretListOptions{
				Filters: filters,
			},
			expectedQueryParams: map[string]string{
				"filters": `{"label":{"label1":true,"label2":true}}`,
			},
		},
	}
	for _, listCase := range listCases {
		client, err := NewClientWithOpts(
			WithVersion("1.25"),
			WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
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
			})),
		)
		assert.NilError(t, err)

		secrets, err := client.SecretList(context.Background(), listCase.options)
		if err != nil {
			t.Fatal(err)
		}
		if len(secrets) != 2 {
			t.Fatalf("expected 2 secrets, got %v", secrets)
		}
	}
}
