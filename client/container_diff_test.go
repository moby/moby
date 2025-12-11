package client

import (
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/container"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerDiffError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerDiff(t.Context(), "nothing", ContainerDiffOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerDiff(t.Context(), "", ContainerDiffOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerDiff(t.Context(), "    ", ContainerDiffOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
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

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, expected)(req)
		}),
	)
	assert.NilError(t, err)

	result, err := client.ContainerDiff(t.Context(), "container_id", ContainerDiffOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.DeepEqual(result.Changes, expected))
}
