package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretListError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SecretList(context.Background(), SecretListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretList(t *testing.T) {
	const expectedURL = "/secrets"

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
				Filters: make(Filters).Add("label", "label1", "label2"),
			},
			expectedQueryParams: map[string]string{
				"filters": `{"label":{"label1":true,"label2":true}}`,
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

			return mockJSONResponse(http.StatusOK, nil, []swarm.Secret{
				{ID: "secret_id1"},
				{ID: "secret_id2"},
			})(req)
		}))
		assert.NilError(t, err)

		res, err := client.SecretList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(res.Items, 2))
	}
}
