package cluster

import (
	"fmt"
	"strings"

	apitypes "github.com/docker/docker/api/types"
	types "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/daemon/cluster/convert"
	swarmapi "github.com/docker/swarmkit/api"
	"golang.org/x/net/context"
)

func getSecretByNameOrIDPrefix(ctx context.Context, state *nodeState, nameOrIDPrefix string) (*swarmapi.Secret, error) {
	// attempt to lookup secret by full ID
	if r, err := state.controlClient.GetSecret(ctx, &swarmapi.GetSecretRequest{
		SecretID: nameOrIDPrefix,
	}); err == nil {
		return r.Secret, nil
	}

	// attempt to lookup secret by full name and partial ID
	// Note here ListSecretRequest_Filters operate with `or`
	r, err := state.controlClient.ListSecrets(ctx, &swarmapi.ListSecretsRequest{
		Filters: &swarmapi.ListSecretsRequest_Filters{
			Names:      []string{nameOrIDPrefix},
			IDPrefixes: []string{nameOrIDPrefix},
		},
	})
	if err != nil {
		return nil, err
	}

	// attempt to lookup secret by full name
	for _, s := range r.Secrets {
		if s.Spec.Annotations.Name == nameOrIDPrefix {
			return s, nil
		}
	}
	// attempt to lookup secret by partial ID (prefix)
	// return error if more than one matches found (ambiguous)
	n := 0
	var found *swarmapi.Secret
	for _, s := range r.Secrets {
		if strings.HasPrefix(s.ID, nameOrIDPrefix) {
			found = s
			n++
		}
	}
	if n > 1 {
		return nil, fmt.Errorf("secret %s is ambiguous (%d matches found)", nameOrIDPrefix, n)
	}
	if found == nil {
		return nil, fmt.Errorf("no such secret: %s", nameOrIDPrefix)
	}
	return found, nil
}

// GetSecret returns a secret from a managed swarm cluster
func (c *Cluster) GetSecret(nameOrIDPrefix string) (types.Secret, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return types.Secret{}, c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	secret, err := getSecretByNameOrIDPrefix(ctx, &state, nameOrIDPrefix)
	if err != nil {
		return types.Secret{}, err
	}
	return convert.SecretFromGRPC(secret), nil
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
func (c *Cluster) RemoveSecret(nameOrIDPrefix string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	state := c.currentNodeState()
	if !state.IsActiveManager() {
		return c.errNoManager(state)
	}

	ctx, cancel := c.getRequestContext()
	defer cancel()

	secret, err := getSecretByNameOrIDPrefix(ctx, &state, nameOrIDPrefix)
	if err != nil {
		return err
	}

	req := &swarmapi.RemoveSecretRequest{
		SecretID: secret.ID,
	}

	_, err = state.controlClient.RemoveSecret(ctx, req)
	return err
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

	_, err := state.controlClient.UpdateSecret(ctx,
		&swarmapi.UpdateSecretRequest{
			SecretID: id,
			SecretVersion: &swarmapi.Version{
				Index: version,
			},
			Spec: &secretSpec,
		})
	return err
}
