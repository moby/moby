package client

import (
	"bytes"
	"context"
	"io"
	"math"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerResizeError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerResize(context.Background(), "container_id", ContainerResizeOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerResize(context.Background(), "", ContainerResizeOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerResize(context.Background(), "    ", ContainerResizeOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerExecResizeError(t *testing.T) {
	client := &Client{
		client: newMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	}
	err := client.ContainerExecResize(context.Background(), "exec_id", ContainerResizeOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestContainerResize(t *testing.T) {
	const expectedURL = "/containers/container_id/resize"

	tests := []struct {
		doc                           string
		opts                          ContainerResizeOptions
		expectedHeight, expectedWidth string
	}{
		{
			doc:            "zero width height", // valid, but not very useful
			opts:           ContainerResizeOptions{},
			expectedWidth:  "0",
			expectedHeight: "0",
		},
		{
			doc: "valid resize",
			opts: ContainerResizeOptions{
				Height: 500,
				Width:  600,
			},
			expectedHeight: "500",
			expectedWidth:  "600",
		},
		{
			doc: "larger than maxint64",
			opts: ContainerResizeOptions{
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
			assert.NilError(t, err)
		})
	}
}

func TestContainerExecResize(t *testing.T) {
	const expectedURL = "/exec/exec_id/resize"
	tests := []struct {
		doc                           string
		opts                          ContainerResizeOptions
		expectedHeight, expectedWidth string
	}{
		{
			doc:            "zero width height", // valid, but not very useful
			opts:           ContainerResizeOptions{},
			expectedWidth:  "0",
			expectedHeight: "0",
		},
		{
			doc: "valid resize",
			opts: ContainerResizeOptions{
				Height: 500,
				Width:  600,
			},
			expectedHeight: "500",
			expectedWidth:  "600",
		},
		{
			doc: "larger than maxint64",
			opts: ContainerResizeOptions{
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
			assert.NilError(t, err)
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
