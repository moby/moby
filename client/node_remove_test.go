package client

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNodeRemoveError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NodeRemove(context.Background(), "node_id", NodeRemoveOptions{Force: false})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.NodeRemove(context.Background(), "", NodeRemoveOptions{Force: false})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.NodeRemove(context.Background(), "    ", NodeRemoveOptions{Force: false})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNodeRemove(t *testing.T) {
	const expectedURL = "/nodes/node_id"

	removeCases := []struct {
		force         bool
		expectedForce string
	}{
		{
			expectedForce: "",
		},
		{
			force:         true,
			expectedForce: "1",
		},
	}

	for _, removeCase := range removeCases {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodDelete, expectedURL); err != nil {
				return nil, err
			}
			force := req.URL.Query().Get("force")
			if force != removeCase.expectedForce {
				return nil, fmt.Errorf("force not set in URL query properly. expected '%s', got %s", removeCase.expectedForce, force)
			}

			return mockResponse(http.StatusOK, nil, "body")(req)
		}))
		assert.NilError(t, err)

		_, err = client.NodeRemove(context.Background(), "node_id", NodeRemoveOptions{Force: removeCase.force})
		assert.NilError(t, err)
	}
}
