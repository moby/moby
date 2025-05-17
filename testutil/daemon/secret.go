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
func (d *Daemon) CreateSecret(tb testing.TB, secretSpec swarm.SecretSpec) string {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	scr, err := cli.SecretCreate(context.Background(), secretSpec)
	assert.NilError(tb, err)

	return scr.ID
}

// ListSecrets returns the list of the current swarm secrets
func (d *Daemon) ListSecrets(tb testing.TB) []swarm.Secret {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	secrets, err := cli.SecretList(context.Background(), types.SecretListOptions{})
	assert.NilError(tb, err)
	return secrets
}

// GetSecret returns a swarm secret identified by the specified id
func (d *Daemon) GetSecret(tb testing.TB, id string) *swarm.Secret {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	secret, _, err := cli.SecretInspectWithRaw(context.Background(), id)
	assert.NilError(tb, err)
	return &secret
}

// DeleteSecret removes the swarm secret identified by the specified id
func (d *Daemon) DeleteSecret(tb testing.TB, id string) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	err := cli.SecretRemove(context.Background(), id)
	assert.NilError(tb, err)
}

// UpdateSecret updates the swarm secret identified by the specified id
// Currently, only label update is supported.
func (d *Daemon) UpdateSecret(tb testing.TB, id string, f ...SecretConstructor) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	secret := d.GetSecret(tb, id)
	for _, fn := range f {
		fn(secret)
	}

	err := cli.SecretUpdate(context.Background(), secret.ID, secret.Version, secret.Spec)

	assert.NilError(tb, err)
}
