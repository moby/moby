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

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerDiffError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	_, err := client.ContainerDiff(context.Background(), "nothing")
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerDiff(t *testing.T) {
	const expectedURL = "/containers/container_id/changes"

	expected := []container.FilesystemChange{
		{
			Kind: container.ChangeModify,
			Path: "/path/1",
		},
		{
			Kind: container.ChangeAdd,
			Path: "/path/2",
		},
		{
			Kind: container.ChangeDelete,
			Path: "/path/3",
		},
	}

	client := &Client{
		client: newMockClient(func(req *http.Request) (*http.Response, error) {
			if !strings.HasPrefix(req.URL.Path, expectedURL) {
				return nil, fmt.Errorf("expected URL '%s', got '%s'", expectedURL, req.URL)
			}
			b, err := json.Marshal(expected)
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	}

	changes, err := client.ContainerDiff(context.Background(), "container_id")
	assert.Check(t, err)
	assert.Check(t, is.DeepEqual(changes, expected))
}
