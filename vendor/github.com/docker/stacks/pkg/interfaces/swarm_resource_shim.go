package interfaces

import (
	"context"

	dockerTypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
)

// SwarmResourceAPIClientShim is an implementation of SwarmResourceBackend
// which uses an underlying client.CommonAPIClient to perform swarm and
// networking operations.
type SwarmResourceAPIClientShim struct {
	dclient client.CommonAPIClient
}

// NewSwarmAPIClientShim creates a new SwarmResourceAPIClientShim from a
// client.CommonAPIClient.
func NewSwarmAPIClientShim(dclient client.CommonAPIClient) SwarmResourceBackend {
	return &SwarmResourceAPIClientShim{
		dclient: dclient,
	}
}

// Info returns the swarm info
func (c *SwarmResourceAPIClientShim) Info() swarm.Info {
	// calls to Info return an error, but there's nothing we can do about that.
	// because Backend call doesn't. So just return the empty Info object even
	// if we get an error.
	info, _ := c.dclient.Info(context.Background())
	return info.Swarm
}

// GetNode returns a specific node by ID.
func (c *SwarmResourceAPIClientShim) GetNode(id string) (swarm.Node, error) {
	node, _, err := c.dclient.NodeInspectWithRaw(context.Background(), id)
	if client.IsErrNotFound(err) {
		return node, errdefs.NotFound(err)
	}
	return node, err
}

// GetServices lists services.
func (c *SwarmResourceAPIClientShim) GetServices(options dockerTypes.ServiceListOptions) ([]swarm.Service, error) {
	return c.dclient.ServiceList(context.Background(), options)
}

// GetService inspects a single service.
func (c *SwarmResourceAPIClientShim) GetService(idOrName string, insertDefaults bool) (swarm.Service, error) {
	svc, _, err := c.dclient.ServiceInspectWithRaw(context.Background(), idOrName, dockerTypes.ServiceInspectOptions{
		InsertDefaults: insertDefaults,
	})
	if client.IsErrNotFound(err) {
		return svc, errdefs.NotFound(err)
	}
	return svc, err
}

// CreateService creates a new service.
func (c *SwarmResourceAPIClientShim) CreateService(spec swarm.ServiceSpec, encodedRegistryAuth string, queryRegistry bool) (*dockerTypes.ServiceCreateResponse, error) {
	resp, err := c.dclient.ServiceCreate(context.Background(), spec, dockerTypes.ServiceCreateOptions{
		EncodedRegistryAuth: encodedRegistryAuth,
		QueryRegistry:       queryRegistry,
	})
	return &resp, err
}

// UpdateService updates a service.
func (c *SwarmResourceAPIClientShim) UpdateService(
	idOrName string,
	version uint64,
	spec swarm.ServiceSpec,
	options dockerTypes.ServiceUpdateOptions,
	queryRegistry bool,
) (*dockerTypes.ServiceUpdateResponse, error) {
	options.QueryRegistry = queryRegistry
	resp, err := c.dclient.ServiceUpdate(context.Background(), idOrName, swarm.Version{Index: version}, spec, options)
	if client.IsErrNotFound(err) {
		return &resp, errdefs.NotFound(err)
	}

	return &resp, err
}

// RemoveService removes a service.
func (c *SwarmResourceAPIClientShim) RemoveService(idOrName string) error {
	return c.dclient.ServiceRemove(context.Background(), idOrName)
}

// GetTasks returns multiple tasks.
func (c *SwarmResourceAPIClientShim) GetTasks(options dockerTypes.TaskListOptions) ([]swarm.Task, error) {
	return c.dclient.TaskList(context.Background(), options)
}

// GetTask returns a task.
func (c *SwarmResourceAPIClientShim) GetTask(taskID string) (swarm.Task, error) {
	task, _, err := c.dclient.TaskInspectWithRaw(context.Background(), taskID)
	if client.IsErrNotFound(err) {
		return task, errdefs.NotFound(err)
	}
	return task, err
}

// GetSecrets lists multiple secrets.
func (c *SwarmResourceAPIClientShim) GetSecrets(opts dockerTypes.SecretListOptions) ([]swarm.Secret, error) {
	return c.dclient.SecretList(context.Background(), opts)
}

// CreateSecret creates a secret.
func (c *SwarmResourceAPIClientShim) CreateSecret(s swarm.SecretSpec) (string, error) {
	resp, err := c.dclient.SecretCreate(context.Background(), s)
	return resp.ID, err
}

// RemoveSecret removes a secret.
func (c *SwarmResourceAPIClientShim) RemoveSecret(idOrName string) error {
	return c.dclient.SecretRemove(context.Background(), idOrName)
}

// GetSecret inspects a secret.
func (c *SwarmResourceAPIClientShim) GetSecret(id string) (swarm.Secret, error) {
	secret, _, err := c.dclient.SecretInspectWithRaw(context.Background(), id)
	if client.IsErrNotFound(err) {
		return secret, errdefs.NotFound(err)
	}
	return secret, err
}

// UpdateSecret updates a secret.
func (c *SwarmResourceAPIClientShim) UpdateSecret(idOrName string, version uint64, spec swarm.SecretSpec) error {
	return c.dclient.SecretUpdate(context.Background(), idOrName, swarm.Version{Index: version}, spec)
}

// GetConfigs lists multiple configs.
func (c *SwarmResourceAPIClientShim) GetConfigs(opts dockerTypes.ConfigListOptions) ([]swarm.Config, error) {
	return c.dclient.ConfigList(context.Background(), opts)
}

// CreateConfig creates a config.
func (c *SwarmResourceAPIClientShim) CreateConfig(s swarm.ConfigSpec) (string, error) {
	resp, err := c.dclient.ConfigCreate(context.Background(), s)
	return resp.ID, err
}

// RemoveConfig removes a config.
func (c *SwarmResourceAPIClientShim) RemoveConfig(id string) error {
	return c.dclient.ConfigRemove(context.Background(), id)
}

// GetConfig inspects a config.
func (c *SwarmResourceAPIClientShim) GetConfig(id string) (swarm.Config, error) {
	cfg, _, err := c.dclient.ConfigInspectWithRaw(context.Background(), id)
	if client.IsErrNotFound(err) {
		return cfg, errdefs.NotFound(err)
	}
	return cfg, err
}

// UpdateConfig updates a config.
func (c *SwarmResourceAPIClientShim) UpdateConfig(idOrName string, version uint64, spec swarm.ConfigSpec) error {
	return c.dclient.ConfigUpdate(context.Background(), idOrName, swarm.Version{Index: version}, spec)
}

// GetNetworks return a list of networks.
func (c *SwarmResourceAPIClientShim) GetNetworks(f filters.Args) ([]dockerTypes.NetworkResource, error) {
	return c.dclient.NetworkList(context.Background(), dockerTypes.NetworkListOptions{
		Filters: f,
	})
}

// GetNetwork inspects a network.
func (c *SwarmResourceAPIClientShim) GetNetwork(name string) (dockerTypes.NetworkResource, error) {
	network, err := c.dclient.NetworkInspect(context.Background(), name, dockerTypes.NetworkInspectOptions{})
	if client.IsErrNotFound(err) {
		return network, errdefs.NotFound(err)
	}

	return network, err
}

// GetNetworksByName is a great example of a bad interface design.
func (c *SwarmResourceAPIClientShim) GetNetworksByName(name string) ([]dockerTypes.NetworkResource, error) {
	f := filters.NewArgs()
	f.Add("name", name)
	return c.GetNetworks(f)
}

// CreateNetwork creates a new network.
func (c *SwarmResourceAPIClientShim) CreateNetwork(nc dockerTypes.NetworkCreateRequest) (string, error) {
	resp, err := c.dclient.NetworkCreate(context.Background(), nc.Name, nc.NetworkCreate)
	return resp.ID, err
}

// RemoveNetwork removes a network.
func (c *SwarmResourceAPIClientShim) RemoveNetwork(name string) error {
	return c.dclient.NetworkRemove(context.Background(), name)
}
