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

func TestServiceListError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ServiceList(context.Background(), ServiceListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestServiceList(t *testing.T) {
	const expectedURL = "/services"

	listCases := []struct {
		options             ServiceListOptions
		expectedQueryParams map[string]string
	}{
		{
			options: ServiceListOptions{},
			expectedQueryParams: map[string]string{
				"filters": "",
			},
		},
		{
			options: ServiceListOptions{
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

			return mockJSONResponse(http.StatusOK, nil, []swarm.Service{
				{ID: "service_id1"},
				{ID: "service_id2"},
			})(req)
		}))
		assert.NilError(t, err)

		list, err := client.ServiceList(context.Background(), listCase.options)
		assert.NilError(t, err)
		assert.Check(t, is.Len(list.Items, 2))
	}
}
