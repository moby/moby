package daemon

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/daemon/container"
	"github.com/moby/moby/v2/daemon/network"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestGetInspectData(t *testing.T) {
	c := &container.Container{
		ID:              "inspect-me",
		NetworkSettings: &network.Settings{},
		HostConfig:      &containertypes.HostConfig{},
		State:           &container.State{},
		ExecCommands:    container.NewExecStore(),
	}

	d := &Daemon{
		linkIndex: newLinkIndex(),
	}
	cfg := &configStore{}
	d.configStore.Store(cfg)

	_, _, err := d.getInspectData(&cfg.Config, c)
	assert.Check(t, err)
}

func TestContainerInspect(t *testing.T) {
	c := &container.Container{
		ID:              "inspect-me",
		NetworkSettings: &network.Settings{},
		HostConfig:      &containertypes.HostConfig{},
		State:           &container.State{},
		ExecCommands:    container.NewExecStore(),
	}

	d := &Daemon{
		linkIndex: newLinkIndex(),
	}
	if d.UsesSnapshotter() {
		t.Skip("does not apply to containerd snapshotters, which don't have RWLayer set")
	}
	cfg := &configStore{}
	d.configStore.Store(cfg)

	_, _, err := d.containerInspect(&cfg.Config, c)
	assert.Check(t, is.ErrorContains(err, "RWLayer of container inspect-me is unexpectedly nil"))

	c.State.Dead = true
	_, _, err = d.containerInspect(&cfg.Config, c)
	assert.Check(t, err)
}
