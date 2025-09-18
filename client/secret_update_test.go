package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretUpdateError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	err = client.SecretUpdate(context.Background(), "secret_id", swarm.Version{}, swarm.SecretSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	err = client.SecretUpdate(context.Background(), "", swarm.Version{}, swarm.SecretSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	err = client.SecretUpdate(context.Background(), "    ", swarm.Version{}, swarm.SecretSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestSecretUpdate(t *testing.T) {
	const expectedURL = "/secrets/secret_id/update"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
		}, nil
	}))
	assert.NilError(t, err)

	err = client.SecretUpdate(context.Background(), "secret_id", swarm.Version{}, swarm.SecretSpec{})
	assert.NilError(t, err)
}
