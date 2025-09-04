package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	err = client.ContainerRemove(context.Background(), "container_id", ContainerRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerRemove(context.Background(), "", ContainerRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerRemove(context.Background(), "    ", ContainerRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerRemoveNotFoundError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusNotFound, "no such container: container_id")))
	assert.NilError(t, err)
	err = client.ContainerRemove(context.Background(), "container_id", ContainerRemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "no such container: container_id"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestContainerRemove(t *testing.T) {
	expectedURL := "/containers/container_id"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
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
	}))
	assert.NilError(t, err)

	err = client.ContainerRemove(context.Background(), "container_id", ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	assert.NilError(t, err)
}
