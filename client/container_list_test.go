package client

import (
	"net/http"
	"net/url"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerListError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerList(t.Context(), ContainerListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerList(t *testing.T) {
	const expectedURL = "/containers/json"

	tests := []struct {
		doc      string
		options  ContainerListOptions
		expected url.Values
	}{
		{
			doc:      "no options",
			expected: url.Values{},
		},
		{
			doc:      "size",
			options:  ContainerListOptions{Size: true},
			expected: url.Values{"size": []string{"1"}},
		},
		{
			doc:      "all",
			options:  ContainerListOptions{All: true},
			expected: url.Values{"all": []string{"1"}},
		},
		{
			doc:      "latest",
			options:  ContainerListOptions{Latest: true}, //nolint:staticcheck // ignore SA1019: field is deprecated.
			expected: url.Values{},
		},
		{
			doc:      "since",
			options:  ContainerListOptions{Since: "container"}, //nolint:staticcheck // ignore SA1019: field is deprecated.
			expected: url.Values{},
		},
		{
			doc:      "before",
			options:  ContainerListOptions{Before: "container"}, //nolint:staticcheck // ignore SA1019: field is deprecated.
			expected: url.Values{},
		},
		{
			doc:      "limit",
			options:  ContainerListOptions{Limit: 1},
			expected: url.Values{"limit": []string{"1"}},
		},
		{
			doc: "filters",
			options: ContainerListOptions{
				Filters: make(Filters).
					Add("label", "label1").
					Add("label", "label2").
					Add("before", "container"),
			},
			expected: url.Values{"filters": []string{`{"before":{"container":true},"label":{"label1":true,"label2":true}}`}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			var query url.Values
			client, err := New(
				WithMockClient(func(req *http.Request) (*http.Response, error) {
					if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
						return nil, err
					}
					query = req.URL.Query()

					return mockJSONResponse(http.StatusOK, nil, []container.Summary{
						{ID: "container_id1"},
						{ID: "container_id2"},
					})(req)
				}),
			)
			assert.NilError(t, err)

			list, err := client.ContainerList(t.Context(), tc.options)
			assert.NilError(t, err)
			assert.Check(t, is.Len(list.Items, 2))
			assert.Check(t, is.DeepEqual(query, tc.expected))
		})
	}
}
