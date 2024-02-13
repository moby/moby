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
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerListError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerList(context.Background(), container.ListOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerList(t *testing.T) {
	expectedURL := "/containers/json"
	expectedFilters := `{"before":{"container":true},"label":{"label1":true,"label2":true}}`
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
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

			b, err := json.Marshal([]types.Container{
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
	}

	containers, err := client.ContainerList(context.Background(), container.ListOptions{
		Size:  true,
		All:   true,
		Since: "container",
		Filters: filters.NewArgs(
			filters.Arg("label", "label1"),
			filters.Arg("label", "label2"),
			filters.Arg("before", "container"),
		),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 2 {
		t.Fatalf("expected 2 containers, got %v", containers)
	}
}
