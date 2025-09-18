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

func TestConfigCreateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestConfigCreate(t *testing.T) {
	const expectedURL = "/configs/create"
	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			b, err := json.Marshal(swarm.ConfigCreateResponse{
				ID: "test_config",
			})
			if err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusCreated,
				Body:       io.NopCloser(bytes.NewReader(b)),
			}, nil
		}),
	)
	assert.NilError(t, err)

	r, err := client.ConfigCreate(context.Background(), swarm.ConfigSpec{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(r.ID, "test_config"))
}
