package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/stretchr/testify/assert"
	"golang.org/x/net/context"
)

func TestContainerRemoveError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{})
	assert.EqualError(t, err, "Error response from daemon: Server error")
}

func TestContainerRemoveNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "missing")),
	}
	err := client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{})
	assert.EqualError(t, err, "Error: No such container: container_id")
	assert.True(t, IsErrNotFound(err))
}

func TestContainerRemove(t *testing.T) {
	expectedURL := "/containers/container_id"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
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
				Body:       ioutil.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.ContainerRemove(context.Background(), "container_id", types.ContainerRemoveOptions{
		RemoveVolumes: true,
		Force:         true,
	})
	assert.NoError(t, err)
}
