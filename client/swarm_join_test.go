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

func TestSwarmJoinError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmJoin(context.Background(), SwarmJoinOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmJoin(t *testing.T) {
	const expectedURL = "/swarm/join"

	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte(""))),
		}, nil
	}))
	assert.NilError(t, err)

	_, err = client.SwarmJoin(context.Background(), SwarmJoinOptions{
		ListenAddr: "0.0.0.0:2377",
	})
	assert.NilError(t, err)
}
