package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerLogsClearError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerLogsClear(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerLogsClearNotFoundError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusNotFound, "Not found")),
	}
	err := client.ContainerLogsClear(context.Background(), "container_id")
	assert.Check(t, is.ErrorType(err, errdefs.IsNotFound))
}

func TestContainerLogsClear(t *testing.T) {
	expectedURL := "/containers/container_id/logs/clear"
	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(""))),
			}, nil
		}),
	}

	err := client.ContainerLogsClear(context.Background(), "container_id")
	if err != nil {
		t.Fatal(err)
	}
}
