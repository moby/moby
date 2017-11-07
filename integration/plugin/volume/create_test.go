// +build linux

package volume

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/volume"
	"github.com/docker/docker/integration-cli/daemon"
)

// TestCreateDerefOnError ensures that if a volume create fails, that the plugin is dereferenced
// Normally 1 volume == 1 reference to a plugin, which prevents a plugin from being removed.
// If the volume create fails, we should make sure to dereference the plugin.
func TestCreateDerefOnError(t *testing.T) {
	t.Parallel()

	d := daemon.New(t, "", dockerdBinary, daemon.Config{})
	d.Start(t)
	defer d.Stop(t)

	c, err := d.NewClient()
	if err != nil {
		t.Fatal(err)
	}

	pName := "testderef"
	createPlugin(t, c, pName, "create-error", asVolumeDriver)

	if err := c.PluginEnable(context.Background(), pName, types.PluginEnableOptions{Timeout: 30}); err != nil {
		t.Fatal(err)
	}

	_, err = c.VolumeCreate(context.Background(), volume.VolumesCreateBody{
		Driver: pName,
		Name:   "fake",
	})
	if err == nil {
		t.Fatal("volume create should have failed")
	}

	if err := c.PluginDisable(context.Background(), pName, types.PluginDisableOptions{}); err != nil {
		t.Fatal(err)
	}

	if err := c.PluginRemove(context.Background(), pName, types.PluginRemoveOptions{}); err != nil {
		t.Fatal(err)
	}
}
