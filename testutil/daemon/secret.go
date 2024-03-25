package daemon

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"gotest.tools/v3/assert"
)

// SecretConstructor defines a swarm secret constructor
type SecretConstructor func(*swarm.Secret)

// CreateSecret creates a secret given the specified spec
func (d *Daemon) CreateSecret(t testing.TB, secretSpec swarm.SecretSpec) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cli := d.NewClientT(t)
	defer cli.Close(ctx)

	scr, err := cli.SecretCreate(ctx, secretSpec)
	assert.NilError(t, err)

	return scr.ID
}

// ListSecrets returns the list of the current swarm secrets
func (d *Daemon) ListSecrets(t testing.TB) []swarm.Secret {
	t.Helper()
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cli := d.NewClientT(t)
	defer cli.Close(ctx)

	secrets, err := cli.SecretList(ctx, types.SecretListOptions{})
	assert.NilError(t, err)
	return secrets
}

// GetSecret returns a swarm secret identified by the specified id
func (d *Daemon) GetSecret(t testing.TB, id string) *swarm.Secret {
	t.Helper()
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cli := d.NewClientT(t)
	defer cli.Close(ctx)

	secret, _, err := cli.SecretInspectWithRaw(context.Background(), id)
	assert.NilError(t, err)
	return &secret
}

// DeleteSecret removes the swarm secret identified by the specified id
func (d *Daemon) DeleteSecret(t testing.TB, id string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cli := d.NewClientT(t)
	defer cli.Close(ctx)

	err := cli.SecretRemove(ctx, id)
	assert.NilError(t, err)
}

// UpdateSecret updates the swarm secret identified by the specified id
// Currently, only label update is supported.
func (d *Daemon) UpdateSecret(t testing.TB, id string, f ...SecretConstructor) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()

	cli := d.NewClientT(t)
	defer cli.Close(ctx)

	secret := d.GetSecret(t, id)
	for _, fn := range f {
		fn(secret)
	}

	err := cli.SecretUpdate(ctx, secret.ID, secret.Version, secret.Spec)

	assert.NilError(t, err)
}
