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
	"github.com/docker/docker/errdefs"
	"github.com/pkg/errors"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerInspectError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}

	_, err := client.ContainerInspect(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerInspectContainerNotFound(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Server error")),
	}

	_, err := client.ContainerInspect(context.Background(), "unknown")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestContainerInspectWithEmptyID(t *testing.T) {
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			return nil, errors.New("should not make request")
		}),
	}
	_, _, err := client.ContainerInspectWithRaw(context.Background(), "", true)
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestContainerInspect(t *testing.T) {
	expectedURL := "/containers/container_id/json"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
		}),
	}

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
