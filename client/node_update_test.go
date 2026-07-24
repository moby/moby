package client

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	cerrdefs "github.com/containerd/errdefs"
	"github.com/moby/moby/api/types/swarm"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestNodeUpdateError(t *testing.T) {
	client, err := New(WithMockClient(errorMock(http.StatusInternalServerError, "Server error")))
	assert.NilError(t, err)

	_, err = client.NodeUpdate(t.Context(), "node_id", NodeUpdateOptions{
		Version: swarm.Version{},
		Spec:    swarm.NodeSpec{},
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInternal))

	_, err = client.NodeUpdate(t.Context(), "", NodeUpdateOptions{
		Version: swarm.Version{},
		Spec:    swarm.NodeSpec{},
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))

	_, err = client.NodeUpdate(t.Context(), "    ", NodeUpdateOptions{
		Version: swarm.Version{},
		Spec:    swarm.NodeSpec{},
	})
	assert.Check(t, is.ErrorType(err, cerrdefs.IsInvalidArgument))
	assert.Check(t, is.ErrorContains(err, "value is empty"))
}

func TestNodeUpdate(t *testing.T) {
	const expectedURL = "/nodes/node_id/update"

	client, err := New(WithMockClient(func(req *http.Request) (*http.Response, error) {
		if err := assertRequest(req, http.MethodPost, expectedURL); err != nil {
			return nil, err
		}
		return mockResponse(http.StatusOK, nil, "body")(req)
	}))
	assert.NilError(t, err)

	_, err = client.NodeUpdate(t.Context(), "node_id", NodeUpdateOptions{
		Version: swarm.Version{},
		Spec:    swarm.NodeSpec{},
	})
	assert.NilError(t, err)
}

func TestNodeUpdateVersionAware(t *testing.T) {
	tests := []struct {
		name           string
		version        string
		opts           NodeUpdateOptions
		expectJSONBody bool // true => expect JSON body with version, false => expect query args
		validateReq    func(t *testing.T, req *http.Request)
	}{
		{
			name:    "API v1.52 should use query args (< v1.53)",
			version: "1.52",
			opts: NodeUpdateOptions{
				Version: swarm.Version{Index: 123},
				Spec: swarm.NodeSpec{
					Availability: swarm.NodeAvailabilityActive,
					Role:         swarm.NodeRoleWorker,
				},
			},
			expectJSONBody: false,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Check(t, is.Equal(req.Method, http.MethodPost))
				assert.Check(t, is.Equal(req.URL.Path, "/v1.52/nodes/node_id/update"))

				// Verify version is in query params
				assert.Check(t, is.Equal(req.URL.Query().Get("version"), "123"))

				// Verify body contains only NodeSpec (not wrapped)
				body, err := io.ReadAll(req.Body)
				assert.NilError(t, err)
				assert.Check(t, len(body) > 0, "body should not be empty")

				var spec swarm.NodeSpec
				err = json.Unmarshal(body, &spec)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(spec.Availability, swarm.NodeAvailabilityActive))
				assert.Check(t, is.Equal(spec.Role, swarm.NodeRoleWorker))
			},
		},
		{
			name:    "API v1.53 should use JSON body (>= v1.53)",
			version: "1.53",
			opts: NodeUpdateOptions{
				Version: swarm.Version{Index: 456},
				Spec: swarm.NodeSpec{
					Availability: swarm.NodeAvailabilityDrain,
					Role:         swarm.NodeRoleManager,
				},
			},
			expectJSONBody: true,
			validateReq: func(t *testing.T, req *http.Request) {
				assert.Check(t, is.Equal(req.Method, http.MethodPost))
				assert.Check(t, is.Equal(req.URL.Path, "/v1.53/nodes/node_id/update"))

				// Verify version is NOT in query params
				_, hasVersion := req.URL.Query()["version"]
				assert.Check(t, !hasVersion, "version should not be in query params for v1.53")

				// Verify body contains NodeUpdateRequest
				body, err := io.ReadAll(req.Body)
				assert.NilError(t, err)
				assert.Check(t, len(body) > 0, "body should not be empty")

				var updateReq NodeUpdateRequest
				err = json.Unmarshal(body, &updateReq)
				assert.NilError(t, err)
				assert.Check(t, is.Equal(updateReq.Version, uint64(456)))
				assert.Check(t, is.Equal(updateReq.Spec.Availability, swarm.NodeAvailabilityDrain))
				assert.Check(t, is.Equal(updateReq.Spec.Role, swarm.NodeRoleManager))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client, err := New(
				WithMockClient(func(req *http.Request) (*http.Response, error) {
					tc.validateReq(t, req)
					return mockResponse(http.StatusOK, nil, "")(req)
				}),
				WithAPIVersion(tc.version),
			)
			assert.NilError(t, err)

			_, err = client.NodeUpdate(t.Context(), "node_id", tc.opts)
			assert.NilError(t, err)
		})
	}
}
