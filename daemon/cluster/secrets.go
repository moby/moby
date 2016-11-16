package cluster

import (
	apitypes "github.com/docker/docker/api/types"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	swarmapi "github.com/docker/swarmkit/api"
)

// GetSecret returns a secret from a managed swarm cluster
func (c *Cluster) GetSecret(id string) (types.Secret, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Secret{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.GetSecret(ctx, &swarmapi.GetSecretRequest{SecretID: id})
	if err != nil {
		return types.Secret{}, err
	}

	return convert.SecretFromGRPC(r.Secret), nil
}

// GetSecrets returns all secrets of a managed swarm cluster.
func (c *Cluster) GetSecrets(options apitypes.SecretListOptions) ([]types.Secret, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return nil, c.errNoManager(state)
	}

	filters, err := newListSecretsFilters(options.Filters)
	if err != nil {
		return nil, err
	}
	ctx, cancel := c.getRequestContext()
	defer cancel()

	r, err := state.controlClient.ListSecrets(ctx,
		&swarmapi.ListSecretsRequest{Filters: filters})
	if err != nil {
		return nil, err
	}

	secrets := []types.Secret{}

	for _, secret := range r.Secrets {
		secrets = append(secrets, convert.SecretFromGRPC(secret))
	}

	return secrets, nil
}

// CreateSecret creates a new secret in a managed swarm cluster.
func (c *Cluster) CreateSecret(s types.SecretSpec) (string, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return "", c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	secretSpec := convert.SecretSpecToGRPC(s)

	r, err := state.controlClient.CreateSecret(ctx,
		&swarmapi.CreateSecretRequest{Spec: &secretSpec})
	if err != nil {
		return "", err
	}

	return r.Secret.ID, nil
}

// RemoveSecret removes a secret from a managed swarm cluster.
func (c *Cluster) RemoveSecret(id string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	req := &swarmapi.RemoveSecretRequest{
		SecretID: id,
	}

	if _, err := state.controlClient.RemoveSecret(ctx, req); err != nil {
		return err
	}
	return nil
}

// UpdateSecret updates a secret in a managed swarm cluster.
// Note: this is not exposed to the CLI but is available from the API only
func (c *Cluster) UpdateSecret(id string, version uint64, spec types.SecretSpec) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	secretSpec := convert.SecretSpecToGRPC(spec)

	if _, err := state.controlClient.UpdateSecret(ctx,
		&swarmapi.UpdateSecretRequest{
			SecretID: id,
			SecretVersion: &swarmapi.Version{
				Index: version,
			},
			Spec: &secretSpec,
		}); err != nil {
		return err
	}

	return nil
}
