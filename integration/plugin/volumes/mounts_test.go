package volumes

import (
	"os"
	"testing"

	plugintypes "github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
	"github.com/moby/moby/v2/testutil/fixtures/plugin"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestPluginWithDevMounts tests very specific regression caused by mounts ordering
// (sorted in the daemon). See #36698
func TestPluginWithDevMounts(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot run daemon when remote daemon")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless)
	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.Start(t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	c := d.NewClientT(t)

	testDir, err := os.MkdirTemp("", "test-dir")
	assert.NilError(t, err)
	defer os.RemoveAll(testDir)

	createPlugin(ctx, t, c, "test", "dummy", asVolumeDriver, func(c *plugin.Config) {
		root := "/"
		dev := "/dev"
		mounts := []plugintypes.Mount{
			{Type: "bind", Source: &root, Destination: "/host", Options: []string{"rbind"}},
			{Type: "bind", Source: &dev, Destination: "/dev", Options: []string{"rbind"}},
			{Type: "bind", Source: &testDir, Destination: "/etc/foo", Options: []string{"rbind"}},
		}
		c.Config.Mounts = append(c.Config.Mounts, mounts...)
		c.PropagatedMount = "/propagated"
		c.Network = plugintypes.NetworkConfig{Type: "host"}
		c.IpcHost = true
	})

	err = c.PluginEnable(ctx, "test", client.PluginEnableOptions{Timeout: 30})
	assert.NilError(t, err)
	defer func() {
		err := c.PluginRemove(ctx, "test", client.PluginRemoveOptions{Force: true})
		assert.Check(t, err)
	}()

	p, _, err := c.PluginInspectWithRaw(ctx, "test")
	assert.NilError(t, err)
	assert.Assert(t, p.Enabled)
}
