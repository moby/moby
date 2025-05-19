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

func TestContainerExportError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerExport(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))

	_, err = client.ContainerExport(context.Background(), "")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerExport(context.Background(), "    ")
	assert.Check(t, is.ErrorType(err, errdefs.IsInvalidParameter))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerExport(t *testing.T) {
	expectedURL := "/containers/container_id/export"
	client := &Client{
		client: newMockClient(func(r *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(r.URL.Path, expectedURL) {
				return nil, fmt.Errorf("Expected URL '%s', got '%s'", expectedURL, r.URL)
			}

			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("response"))),
			}, nil
		}),
	}
	body, err := client.ContainerExport(context.Background(), "container_id")
	assert.NilError(t, err)
	defer body.Close()
	content, err := io.ReadAll(body)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(content), "response"))
}
