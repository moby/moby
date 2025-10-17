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

func TestConfigUpdateError(t *testing.T) {
	client, err := NewClientWithOpts(
		WithMockClient(errorMock(http.StatusInternalServerError, "Server error")),
	)
	assert.NilError(t, err)

	_, err = client.ConfigUpdate(context.Background(), "config_id", ConfigUpdateOptions{Version: swarm.Version{}, Config: swarm.ConfigSpec{}})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ConfigUpdate(context.Background(), "", ConfigUpdateOptions{Version: swarm.Version{}, Config: swarm.ConfigSpec{}})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ConfigUpdate(context.Background(), "    ", ConfigUpdateOptions{Version: swarm.Version{}, Config: swarm.ConfigSpec{}})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestConfigUpdate(t *testing.T) {
	const expectedURL = "/configs/config_id/update"

	client, err := NewClientWithOpts(
		WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte("body"))),
			}, nil
		}),
	)
	assert.NilError(t, err)

	_, err = client.ConfigUpdate(context.Background(), "config_id", ConfigUpdateOptions{Version: swarm.Version{}, Config: swarm.ConfigSpec{}})
	assert.NilError(t, err)
}
