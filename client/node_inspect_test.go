package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNodeInspectError(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NodeInspect(context.Background(), "nothing", NodeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestNodeInspectNodeNotFound(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(errorMock(http.StatusNotFound, "Server error")))
	assert.NilError(t, err)

	_, err = client.NodeInspect(context.Background(), "unknown", NodeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsNotFound))
}

func TestNodeInspectWithEmptyID(t *testing.T) {
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		return nil, errors.New("should not make request")
	}))
	assert.NilError(t, err)
	_, err = client.NodeInspect(context.Background(), "", NodeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.NodeInspect(context.Background(), "    ", NodeInspectOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNodeInspect(t *testing.T) {
	const expectedURL = "/nodes/node_id"
	client, err := NewClientWithOpts(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodGet, expectedURL); err != nil {
			return nil, err
		}
		content, err := json.Marshal(swarm.Node{
			ID: "node_id",
		})
		if err != nil {
			return nil, err
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(content)),
		}, nil
	}))
	assert.NilError(t, err)

	result, err := client.NodeInspect(context.Background(), "node_id", NodeInspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(result.Node.ID, "node_id"))
}
