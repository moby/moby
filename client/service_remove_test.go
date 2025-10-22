package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestServiceRemoveError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.ServiceRemove(context.Background(), "service_id", ServiceRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ServiceRemove(context.Background(), "", ServiceRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ServiceRemove(context.Background(), "    ", ServiceRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestServiceRemoveNotFoundError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusNotFound, "no such service: service_id")))
	assert.NilError(t, err)

	_, err = client.ServiceRemove(context.Background(), "service_id", ServiceRemoveOptions{})
	assert.Check(t, is.ErrorContains(err, "no such service: service_id"))
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestServiceRemove(t *testing.T) {
	const expectedURL = "/services/service_id"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
		}, nil
	}))
	assert.NilError(t, err)

	_, err = client.ServiceRemove(context.Background(), "service_id", ServiceRemoveOptions{})
	assert.NilError(t, err)
}
