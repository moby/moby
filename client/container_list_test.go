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
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerListError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerList(context.Background(), ContainerListOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerList(t *testing.T) {
	const (
		expectedURL     = "/containers/json"
		expectedFilters = `{"before":{"container":true},"label":{"label1":true,"label2":true}}`
	)
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			query := req.URL.Query()
			all := query.Get("all")
			if all != "1" {
				return nil, fmt.Errorf("all not set in URL query properly. Expected '1', got %s", all)
			}
			limit := query.Get("limit")
			if limit != "" {
				return nil, fmt.Errorf("limit should have not be present in query, got %s", limit)
			}
			since := query.Get("since")
			if since != "container" {
				return nil, fmt.Errorf("since not set in URL query properly. Expected 'container', got %s", since)
			}
			before := query.Get("before")
			if before != "" {
				return nil, fmt.Errorf("before should have not be present in query, got %s", before)
			}
			size := query.Get("size")
			if size != "1" {
				return nil, fmt.Errorf("size not set in URL query properly. Expected '1', got %s", size)
			}
			fltrs := query.Get("filters")
			if fltrs != expectedFilters {
				return nil, fmt.Errorf("expected filters incoherent '%v' with actual filters %v", expectedFilters, fltrs)
			}

			b, err := json.Marshal([]container.Summary{
				{
					ID: "container_id1",
				},
				{
					ID: "container_id2",
				},
			})
			if err != nil {
				return nil, err
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	containers, err := client.ContainerList(context.Background(), ContainerListOptions{
		Size:  true,
		All:   true,
		Since: "container",
		Filters: make(Filters).
			Add("label", "label1").
			Add("label", "label2").
			Add("before", "container"),
	})
	assert.NilError(t, err)
	assert.Check(t, is.Len(containers, 2))
}
