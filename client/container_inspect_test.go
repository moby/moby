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

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
)

func TestContainerInspectError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.ContainerInspect(context.Background(), "nothing")
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerInspectContainerNotFound(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "Server error"))),
	)
	assert.NilError(t, err)

	_, err = client.ContainerInspect(context.Background(), "unknown")
	if err == nil || !IsErrNotFound(err) {
		t.Fatalf("expected a containerNotFound error, got %v", err)
	}
}

func TestContainerInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		})),
	)
	assert.NilError(t, err)
	_, _, err = client.ContainerInspectWithRaw(context.Background(), "", true)
	if !IsErrNotFound(err) {
		t.Fatalf("Expected NotFoundError, got %v", err)
	}
}

func TestContainerInspect(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id/json"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			content, err := json.Marshal(types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID:    "container_id",
					Image: "image",
					Name:  "name",
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

	r, err := client.ContainerInspect(context.Background(), "container_id")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "container_id" {
		t.Fatalf("expected `container_id`, got %s", r.ID)
	}
	if r.Image != "image" {
		t.Fatalf("expected `image`, got %s", r.Image)
	}
	if r.Name != "name" {
		t.Fatalf("expected `name`, got %s", r.Name)
	}
}

// TestContainerInspectNode tests that the "Node" field is included in the "inspect"
// output. This information is only present when connected to a Swarm standalone API.
func TestContainerInspectNode(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			content, err := json.Marshal(types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID:    "container_id",
					Image: "image",
					Name:  "name",
					Node: &types.ContainerNode{
						ID:     "container_node_id",
						Addr:   "container_node",
						Labels: map[string]string{"foo": "bar"},
					},
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

	r, err := client.ContainerInspect(context.Background(), "container_id")
	if err != nil {
		t.Fatal(err)
	}
	if r.ID != "container_id" {
		t.Fatalf("expected `container_id`, got %s", r.ID)
	}
	if r.Image != "image" {
		t.Fatalf("expected `image`, got %s", r.Image)
	}
	if r.Name != "name" {
		t.Fatalf("expected `name`, got %s", r.Name)
	}
	if r.Node.ID != "container_node_id" {
		t.Fatalf("expected `container_node_id`, got %s", r.Node.ID)
	}
	if r.Node.Addr != "container_node" {
		t.Fatalf("expected `container_node`, got %s", r.Node.Addr)
	}
	foo, ok := r.Node.Labels["foo"]
	if foo != "bar" || !ok {
		t.Fatalf("expected `bar` for label `foo`")
	}
}
