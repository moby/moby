package daemon

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/swarm"
	"gotest.tools/v3/assert"
)

// ConfigConstructor defines a swarm config constructor
type ConfigConstructor func(*swarm.Config)

// CreateConfig creates a config given the specified spec
func (d *Daemon) CreateConfig(t testing.TB, configSpec swarm.ConfigSpec) string {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	scr, err := cli.ConfigCreate(context.Background(), configSpec)
	assert.NilError(t, err)
	return scr.ID
}

// ListConfigs returns the list of the current swarm configs
func (d *Daemon) ListConfigs(t testing.TB) []swarm.Config {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	configs, err := cli.ConfigList(context.Background(), types.ConfigListOptions{})
	assert.NilError(t, err)
	return configs
}

// GetConfig returns a swarm config identified by the specified id
func (d *Daemon) GetConfig(t testing.TB, id string) *swarm.Config {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	config, _, err := cli.ConfigInspectWithRaw(context.Background(), id)
	assert.NilError(t, err)
	return &config
}

// DeleteConfig removes the swarm config identified by the specified id
func (d *Daemon) DeleteConfig(t testing.TB, id string) {
	t.Helper()
	cli := d.NewClientT(t)
	defer cli.Close()

	err := cli.ConfigRemove(context.Background(), id)
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

	err := cli.ConfigUpdate(context.Background(), config.ID, config.Version, config.Spec)
	assert.NilError(t, err)
}
