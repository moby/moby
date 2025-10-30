package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestConfigRemoveError(t *testing.T) {
	client, err := New(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigRemove(context.Background(), "config_id", ConfigRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ConfigRemove(context.Background(), "", ConfigRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ConfigRemove(context.Background(), "    ", ConfigRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestConfigRemove(t *testing.T) {
	const expectedURL = "/configs/config_id"

	client, err := New(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
				return nil, err
			}
			return mockJSONResponse(http.StatusOK, nil, "")(req)
		}),
	)
	assert.NilError(t, err)

	_, err = client.ConfigRemove(context.Background(), "config_id", ConfigRemoveOptions{})
	assert.NilError(t, err)
}
