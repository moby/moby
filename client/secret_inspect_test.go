package client

import (
	"context"
	"errors"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretInspectError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SecretInspect(context.Background(), "nothing", SecretInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretInspectSecretNotFound(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.SecretInspect(context.Background(), "unknown", SecretInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestSecretInspectWithEmptyID(t *testing.T) {
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.SecretInspect(context.Background(), "", SecretInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.SecretInspect(context.Background(), "    ", SecretInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestSecretInspect(t *testing.T) {
	const expectedURL = "/secrets/secret_id"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusOK, nil, swarm.Secret{
			ID: "secret_id",
		})(req)
	}))
	assert.NilError(t, err)

	res, err := client.SecretInspect(context.Background(), "secret_id", SecretInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.Secret.ID, "secret_id"))
}
