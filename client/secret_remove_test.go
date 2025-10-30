package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretRemoveError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SecretRemove(context.Background(), "secret_id", SecretRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.SecretRemove(context.Background(), "", SecretRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.SecretRemove(context.Background(), "   ", SecretRemoveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestSecretRemove(t *testing.T) {
	const expectedURL = "/secrets/secret_id"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, "body")(req)
	}))
	assert.NilError(t, err)

	_, err = client.SecretRemove(context.Background(), "secret_id", SecretRemoveOptions{})
	assert.NilError(t, err)
}
