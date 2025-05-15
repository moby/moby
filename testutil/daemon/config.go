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
func (d *Daemon) CreateConfig(tb testing.TB, configSpec swarm.ConfigSpec) string {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	scr, err := cli.ConfigCreate(context.Background(), configSpec)
	assert.NilError(tb, err)
	return scr.ID
}

// ListConfigs returns the list of the current swarm configs
func (d *Daemon) ListConfigs(tb testing.TB) []swarm.Config {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	configs, err := cli.ConfigList(context.Background(), types.ConfigListOptions{})
	assert.NilError(tb, err)
	return configs
}

// GetConfig returns a swarm config identified by the specified id
func (d *Daemon) GetConfig(tb testing.TB, id string) *swarm.Config {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	config, _, err := cli.ConfigInspectWithRaw(context.Background(), id)
	assert.NilError(tb, err)
	return &config
}

// DeleteConfig removes the swarm config identified by the specified id
func (d *Daemon) DeleteConfig(tb testing.TB, id string) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	err := cli.ConfigRemove(context.Background(), id)
	assert.NilError(tb, err)
}

// UpdateConfig updates the swarm config identified by the specified id
// Currently, only label update is supported.
func (d *Daemon) UpdateConfig(tb testing.TB, id string, f ...ConfigConstructor) {
	tb.Helper()
	cli := d.NewClientT(tb)
	defer cli.Close()

	config := d.GetConfig(tb, id)
	for _, fn := range f {
		fn(config)
	}

	err := cli.ConfigUpdate(context.Background(), config.ID, config.Version, config.Spec)
	assert.NilError(tb, err)
}
