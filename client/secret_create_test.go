package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestSecretCreateError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.SecretCreate(context.Background(), swarm.SecretSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSecretCreate(t *testing.T) {
	const expectedURL = "/secrets/create"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		b, err := json.Marshal(swarm.SecretCreateResponse{
			ID: "test_secret",
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusCreated,
			Body:       io.NopCloser(bytes.NewReader(b)),
		}, nil
	}))
	assert.NilError(t, err)

	r, err := client.SecretCreate(context.Background(), swarm.SecretSpec{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "test_secret"))
}
