package client

import (
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerExportError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ContainerExport(t.Context(), "nothing", ContainerExportOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerExport(t.Context(), "", ContainerExportOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerExport(t.Context(), "    ", ContainerExportOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerExport(t *testing.T) {
	const expectedURL = "/containers/container_id/export"
	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
				return nil, err
			}
			return mockResponse(http.StatusOK, nil, "response")(req)
		}),
	)
	assert.NilError(t, err)
	res, err := client.ContainerExport(t.Context(), "container_id", ContainerExportOptions{})
	assert.NilError(t, err)
	defer res.Close()
	content, err := io.ReadAll(res)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(string(content), "response"))
}
