package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerRenameError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	err = client.ContainerRename(context.Background(), "nothing", "newNothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.ContainerRename(context.Background(), "", "newNothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.ContainerRename(context.Background(), "    ", "newNothing")
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerRename(t *testing.T) {
	const expectedURL = "/containers/container_id/rename"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		name := req.URL.Query().Get("name")
		if name != "newName" {
			return nil, fmt.Errorf("name not set in URL query properly. Expected 'newName', got %s", name)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}))
	assert.NilError(t, err)

	err = client.ContainerRename(context.Background(), "container_id", "newName")
	assert.NilError(t, err)
}
