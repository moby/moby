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

func TestSwarmLeaveError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.SwarmLeave(context.Background(), SwarmLeaveOptions{})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))
}

func TestSwarmLeave(t *testing.T) {
	const expectedURL = "/swarm/leave"

	leaveCases := []struct {
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

	for _, leaveCase := range leaveCases {
		client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
			if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
				return nil, err
			}
			force := req.URL.Query().Get("force")
			if force != leaveCase.expectedForce {
				return nil, fmt.Errorf("force not set in URL query properly. expected '%s', got %s", leaveCase.expectedForce, force)
			}
			return mockResponse(http.StatusOK, nil, "")(req)
		}))
		assert.NilError(t, err)

		_, err = client.SwarmLeave(context.Background(), SwarmLeaveOptions{Force: leaveCase.force})
		assert.NilError(t, err)
	}
}
