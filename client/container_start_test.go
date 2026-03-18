package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestContainerStartError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)
	_, err = client.ContainerStart(t.Context(), "nothing", ContainerStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.ContainerStart(t.Context(), "", ContainerStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.ContainerStart(t.Context(), "    ", ContainerStartOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestContainerStart(t *testing.T) {
	const expectedURL = "/containers/container_id/start"
	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		// we're not expecting any payload, but if one is supplied, check it is valid.
		if req.Header.Get("Content-Type") == "application/json" {
			var startConfig any
			if err := json.NewDecoder(req.Body).Decode(&startConfig); err != nil {
				return nil, fmt.Errorf("Unable to parse json: %s", err)
			}
		}

		checkpoint := req.URL.Query().Get("checkpoint")
		if checkpoint != "checkpoint_id" {
			return nil, fmt.Errorf("checkpoint not set in URL query properly. Expected 'checkpoint_id', got %s", checkpoint)
		}
		return mockResponse(http.StatusOK, nil, "")(req)
	}))
	assert.NilError(t, err)

	_, err = client.ContainerStart(t.Context(), "container_id", ContainerStartOptions{CheckpointID: "checkpoint_id"})
	assert.NilError(t, err)
}
