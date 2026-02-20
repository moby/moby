package client

import (
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/image"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestImageListError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ImageList(t.Context(), ImageListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

// TestImageListConnectionError verifies that connection errors occurring
// during API-version negotiation are not shadowed by API-version errors.
//
// Regression test for https://github.com/docker/cli/issues/4890
func TestImageListConnectionError(t *testing.T) {
	client, err := New(WithHost("tcp://no-such-host.invalid"))
	assert.NilError(t, err)

	_, err = client.ImageList(t.Context(), ImageListOptions{})
	assert.Check(t, is.ErrorType(err, IsErrConnectionFailed))
}

func TestImageList(t *testing.T) {
	const expectedURL = "/images/json"

	tests := []struct {
		doc                 string
		options             ImageListOptions
		expectedQueryParams map[string]string
	}{
		{
			doc:     "no options",
			options: ImageListOptions{},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": "",
			},
		},
		{
			doc: "label filters and dangling",
			options: ImageListOptions{
				Filters: make(Filters).
					Add("label", "label1").
					Add("label", "label2").
					Add("dangling", "true"),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"true":true},"label":{"label1":true,"label2":true}}`,
			},
		},
		{
			doc: "label filters no dangling",
			options: ImageListOptions{
				Filters: make(Filters).Add("dangling", "false"),
			},
			expectedQueryParams: map[string]string{
				"all":     "",
				"filter":  "",
				"filters": `{"dangling":{"false":true}}`,
			},
		},
		{
			doc: "with shared size",
			options: ImageListOptions{
				SharedSize: true,
			},
			expectedQueryParams: map[string]string{
				"shared-size": "1",
			},
		},
		{
			doc: "with identity",
			options: ImageListOptions{
				Identity: true,
			},
			expectedQueryParams: map[string]string{
				"identity": "1",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
				if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
					return nil, err
				}
				query := req.URL.Query()
				for key, expected := range tc.expectedQueryParams {
					actual := query.Get(key)
					if actual != expected {
						return nil, fmt.Errorf("%s not set in URL query properly. Expected '%s', got %s", key, expected, actual)
					}
				}
				return mockJSONResponse(http.StatusOK, nil, []image.Summary{
					{ID: "image_id2"},
					{ID: "image_id2"},
				})(req)
			}))
			assert.NilError(t, err)
			defer func() { _ = client.Close() }()

			images, err := client.ImageList(t.Context(), tc.options)
			assert.NilError(t, err)
			assert.Check(t, is.Len(images.Items, 2))
		})
	}
}
