package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
)

func TestContainerRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusInternalServerError, "Server error"))),
	)
	assert.NilError(t, err)
	err = client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{})
	if !errdefs.IsSystem(err) {
		t.Fatalf("expected a Server Error, got %[1]T: %[1]v", err)
	}
}

func TestContainerRemoveNotFoundError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(errorMock(http.StatusNotFound, "no such container: container_id"))),
	)
	assert.NilError(t, err)
	err = client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{})
	assert.ErrorContains(t, err, "no such container: container_id")
	assert.Check(t, IsErrNotFound(err))
}

func TestContainerRemove(t *testing.T) {
	expectedURL := "/v" + api.DefaultVersion + "/containers/container_id"
	client, err := NewClientWithOpts(
		WithHTTPClient(newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			query := req.URL.Query()
			volume := query.Get("v")
			if volume != "1" {
				return nil, fmt.Errorf("v (volume) not set in URL query properly. Expected '1', got %s", volume)
			}
			force := query.Get("force")
			if force != "1" {
				return nil, fmt.Errorf("force not set in URL query properly. Expected '1', got %s", force)
			}
			link := query.Get("link")
			if link != "" {
				return nil, fmt.Errorf("link should have not be present in query, go %s", link)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		})),
	)
	assert.NilError(t, err)

	err = client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	assert.Check(t, err)
}
