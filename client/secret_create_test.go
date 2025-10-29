package client

import (
	"context"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretCreateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.SecretCreate(context.Background(), SecretCreateOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretCreate(t *testing.T) {
	const expectedURL = "/secrets/create"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockJSONResponse(http.StatusCreated, nil, swarm.SecretCreateResponse{
			ID: "test_secret",
		})(req)
	}))
	assert.NilError(t, err)

	r, err := client.SecretCreate(context.Background(), SecretCreateOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "test_secret"))
}
