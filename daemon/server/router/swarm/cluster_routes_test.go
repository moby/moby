package swarm

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/v2/daemon/server/backend"
	"github.com/moby/moby/v2/daemon/server/httputils"
	"github.com/moby/moby/v2/daemon/server/swarmbackend"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

type mockSwarmBackend struct {
	updateNodeID      string
	updateNodeVersion uint64
	updateNodeSpec    swarm.NodeSpec
	updateNodeErr     error
}

func (m *mockSwarmBackend) Init(req swarm.InitRequest) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) Join(req swarm.JoinRequest) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) Leave(ctx context.Context, force bool) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) Inspect() (swarm.Swarm, error) {
	return swarm.Swarm{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) Update(version uint64, spec swarm.Spec, flags swarmbackend.UpdateFlags) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetUnlockKey() (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) UnlockSwarm(req swarm.UnlockRequest) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetServices(opts swarmbackend.ServiceListOptions) ([]swarm.Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetService(idOrName string, insertDefaults bool) (swarm.Service, error) {
	return swarm.Service{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) CreateService(spec swarm.ServiceSpec, auth string, queryRegistry bool) (*swarm.ServiceCreateResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) UpdateService(id string, version uint64, spec swarm.ServiceSpec, opts swarmbackend.ServiceUpdateOptions, queryRegistry bool) (*swarm.ServiceUpdateResponse, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) RemoveService(id string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) ServiceLogs(ctx context.Context, selector *backend.LogSelector, opts *backend.ContainerLogsOptions) (<-chan *backend.LogMessage, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetNodes(opts swarmbackend.NodeListOptions) ([]swarm.Node, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetNode(id string) (swarm.Node, error) {
	return swarm.Node{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) UpdateNode(id string, version uint64, spec swarm.NodeSpec) error {
	m.updateNodeID = id
	m.updateNodeVersion = version
	m.updateNodeSpec = spec
	return m.updateNodeErr
}

func (m *mockSwarmBackend) RemoveNode(id string, force bool) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetTasks(opts swarmbackend.TaskListOptions) ([]swarm.Task, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetTask(id string) (swarm.Task, error) {
	return swarm.Task{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetSecrets(opts swarmbackend.SecretListOptions) ([]swarm.Secret, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) CreateSecret(s swarm.SecretSpec) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) RemoveSecret(idOrName string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetSecret(id string) (swarm.Secret, error) {
	return swarm.Secret{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) UpdateSecret(idOrName string, version uint64, spec swarm.SecretSpec) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetConfigs(opts swarmbackend.ConfigListOptions) ([]swarm.Config, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) CreateConfig(s swarm.ConfigSpec) (string, error) {
	return "", fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) RemoveConfig(id string) error {
	return fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) GetConfig(id string) (swarm.Config, error) {
	return swarm.Config{}, fmt.Errorf("not implemented")
}

func (m *mockSwarmBackend) UpdateConfig(idOrName string, version uint64, spec swarm.ConfigSpec) error {
	return fmt.Errorf("not implemented")
}

func TestUpdateNode(t *testing.T) {
	testcases := []struct {
		name        string
		apiVersion  string
		requestBody string
		queryParams url.Values
		nodeID      string
		validate    func(t *testing.T, backend *mockSwarmBackend)
		expError    string
	}{
		{
			name:       "API v1.53 JSON body with version and spec",
			apiVersion: "1.53",
			nodeID:     "node123",
			requestBody: `{
				"version": 456,
				"spec": {
					"Availability": "active",
					"Role": "worker"
				}
			}`,
			validate: func(t *testing.T, backend *mockSwarmBackend) {
				assert.Check(t, is.Equal(backend.updateNodeID, "node123"))
				assert.Check(t, is.Equal(backend.updateNodeVersion, uint64(456)))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Availability, swarm.NodeAvailabilityActive))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Role, swarm.NodeRoleWorker))
			},
		},
		{
			name:       "API v1.53 JSON body drain availability",
			apiVersion: "1.53",
			nodeID:     "node789",
			requestBody: `{
				"version": 123,
				"spec": {
					"Availability": "drain",
					"Role": "manager"
				}
			}`,
			validate: func(t *testing.T, backend *mockSwarmBackend) {
				assert.Check(t, is.Equal(backend.updateNodeID, "node789"))
				assert.Check(t, is.Equal(backend.updateNodeVersion, uint64(123)))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Availability, swarm.NodeAvailabilityDrain))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Role, swarm.NodeRoleManager))
			},
		},
		{
			name:        "API v1.53 invalid JSON",
			apiVersion:  "1.53",
			nodeID:      "node123",
			requestBody: `{"version": "not a number"}`,
			expError:    "json:",
		},
		{
			name:        "API v1.53 malformed JSON",
			apiVersion:  "1.53",
			nodeID:      "node123",
			requestBody: `{"version": 123, "spec": {`,
			expError:    "unexpected EOF",
		},
		{
			name:       "API v1.52 query args with version",
			apiVersion: "1.52",
			nodeID:     "node456",
			queryParams: url.Values{
				"version": []string{"789"},
			},
			requestBody: `{
				"Availability": "pause",
				"Role": "worker"
			}`,
			validate: func(t *testing.T, backend *mockSwarmBackend) {
				assert.Check(t, is.Equal(backend.updateNodeID, "node456"))
				assert.Check(t, is.Equal(backend.updateNodeVersion, uint64(789)))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Availability, swarm.NodeAvailabilityPause))
				assert.Check(t, is.Equal(backend.updateNodeSpec.Role, swarm.NodeRoleWorker))
			},
		},
		{
			name:       "API v1.52 invalid version query param",
			apiVersion: "1.52",
			nodeID:     "node123",
			queryParams: url.Values{
				"version": []string{"not-a-number"},
			},
			requestBody: `{"Availability": "active"}`,
			expError:    "invalid node version",
		},
		{
			name:       "API v1.52 missing version query param",
			apiVersion: "1.52",
			nodeID:     "node123",
			queryParams: url.Values{},
			requestBody: `{"Availability": "active"}`,
			expError:    "invalid node version",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			backend := &mockSwarmBackend{}
			router := &swarmRouter{backend: backend}

			req := httptest.NewRequest(http.MethodPost, "/nodes/"+tc.nodeID+"/update", bytes.NewReader([]byte(tc.requestBody)))
			req.Header.Set("Content-Type", "application/json")

			if tc.queryParams != nil {
				req.URL.RawQuery = tc.queryParams.Encode()
			}

			ctx := context.WithValue(context.Background(), httputils.APIVersionKey{}, tc.apiVersion)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			vars := map[string]string{"id": tc.nodeID}
			err := router.updateNode(ctx, w, req, vars)

			if tc.expError != "" {
				assert.Check(t, err != nil, "expected error but got nil")
				assert.Check(t, is.ErrorContains(err, tc.expError))
				return
			}

			assert.NilError(t, err)

			if tc.validate != nil {
				tc.validate(t, backend)
			}
		})
	}
}

func TestUpdateNodeBackendError(t *testing.T) {
	backend := &mockSwarmBackend{
		updateNodeErr: fmt.Errorf("backend error: node update failed"),
	}
	router := &swarmRouter{backend: backend}

	req := httptest.NewRequest(http.MethodPost, "/nodes/node123/update", bytes.NewReader([]byte(`{"version":1,"spec":{}}`)))
	req.Header.Set("Content-Type", "application/json")

	ctx := context.WithValue(context.Background(), httputils.APIVersionKey{}, "1.53")
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()

	vars := map[string]string{"id": "node123"}
	err := router.updateNode(ctx, w, req, vars)
	assert.Check(t, err != nil)
	assert.Check(t, is.ErrorContains(err, "node update failed"))
}
