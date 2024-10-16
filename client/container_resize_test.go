package client // import "github.com/docker/docker/client"

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerResizeError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerResize(context.Background(), "container_id", container.ResizeOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerExecResizeError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerExecResize(context.Background(), "exec_id", container.ResizeOptions{})
	assert.Check(t, is.ErrorType(err, errdefs.IsSystem))
}

func TestContainerResize(t *testing.T) {
	const expectedURL = "/containers/container_id/resize"

	tests := []struct {
		doc                           string
		opts                          container.ResizeOptions
		expectedHeight, expectedWidth string
	}{
		{
			doc:            "zero width height", // valid, but not very useful
			opts:           container.ResizeOptions{},
			expectedWidth:  "0",
			expectedHeight: "0",
		},
		{
			doc: "valid resize",
			opts: container.ResizeOptions{
				Height: 500,
				Width:  600,
			},
			expectedHeight: "500",
			expectedWidth:  "600",
		},
		{
			doc: "larger than maxint64",
			opts: container.ResizeOptions{
				Height: math.MaxInt64 + 1,
				Width:  math.MaxInt64 + 2,
			},
			expectedHeight: "9223372036854775808",
			expectedWidth:  "9223372036854775809",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(resizeTransport(t, expectedURL, tc.expectedHeight, tc.expectedWidth)),
			}
			err := client.ContainerResize(context.Background(), "container_id", tc.opts)
			assert.Check(t, err)
		})
	}
}

func TestContainerExecResize(t *testing.T) {
	const expectedURL = "/exec/exec_id/resize"
	tests := []struct {
		doc                           string
		opts                          container.ResizeOptions
		expectedHeight, expectedWidth string
	}{
		{
			doc:            "zero width height", // valid, but not very useful
			opts:           container.ResizeOptions{},
			expectedWidth:  "0",
			expectedHeight: "0",
		},
		{
			doc: "valid resize",
			opts: container.ResizeOptions{
				Height: 500,
				Width:  600,
			},
			expectedHeight: "500",
			expectedWidth:  "600",
		},
		{
			doc: "larger than maxint64",
			opts: container.ResizeOptions{
				Height: math.MaxInt64 + 1,
				Width:  math.MaxInt64 + 2,
			},
			expectedHeight: "9223372036854775808",
			expectedWidth:  "9223372036854775809",
		},
	}
	for _, tc := range tests {
		t.Run(tc.doc, func(t *testing.T) {
			client := &Client{
				client: newMockClient(resizeTransport(t, expectedURL, tc.expectedHeight, tc.expectedWidth)),
			}
			err := client.ContainerExecResize(context.Background(), "exec_id", tc.opts)
			assert.Check(t, err)
		})
	}
}

func resizeTransport(t *testing.T, expectedURL, expectedHeight, expectedWidth string) func(req *http.Request) (*http.Response, error) {
	return func(req *http.Request) (*http.Response, error) {
		assert.Check(t, is.Equal(req.URL.Path, expectedURL))

		query := req.URL.Query()
		assert.Check(t, is.Equal(query.Get("h"), expectedHeight))
		assert.Check(t, is.Equal(query.Get("w"), expectedWidth))
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}
}
