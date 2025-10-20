package daemon

import (
	"context"
	"testing"

	"github.com/moby/moby/api/types/swarm"
	"github.com/moby/moby/client"
	"gotest.tools/v3/assert"
)

// ConfigConstructor defines a swarm config constructor
type ConfigConstructor func(*swarm.Config)

// CreateConfig creates a config given the specified spec
func (d *Daemon) CreateConfig(t testing.TB, configSpec swarm.ConfigSpec) string {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.ConfigCreate(context.Background(), client.ConfigCreateOptions{
		Spec: configSpec,
	})
	assert.NilError(t, err)
	return result.ID
}

// ListConfigs returns the list of the current swarm configs
func (d *Daemon) ListConfigs(t testing.TB) []swarm.Config {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.ConfigList(context.Background(), client.ConfigListOptions{})
	assert.NilError(t, err)
	return result.Configs
}

// GetConfig returns a swarm config identified by the specified id
func (d *Daemon) GetConfig(t testing.TB, id string) *swarm.Config {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	result, err := cli.ConfigInspect(context.Background(), id, client.ConfigInspectOptions{})
	assert.NilError(t, err)
	return &result.Config
}

// DeleteConfig removes the swarm config identified by the specified id
func (d *Daemon) DeleteConfig(t testing.TB, id string) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	_, err := cli.ConfigRemove(context.Background(), id, client.ConfigRemoveOptions{})
	assert.NilError(t, err)
}

// UpdateConfig updates the swarm config identified by the specified id
// Currently, only label update is supported.
func (d *Daemon) UpdateConfig(t testing.TB, id string, f ...ConfigConstructor) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	config := d.GetConfig(t, id)
	for _, fn := range f {
		fn(config)
	}

	_, err := cli.ConfigUpdate(context.Background(), config.ID, client.ConfigUpdateOptions{Version: config.Version, Spec: config.Spec})
	assert.NilError(t, err)
}
