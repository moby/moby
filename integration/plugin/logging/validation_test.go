package logging

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration-cli/daemon"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/skip"
)

// Regression test for #35553
// Ensure that a daemon with a log plugin set as the default logger for containers
// does not keep the daemon from starting.
func TestDaemonStartWithLogOpt(t *testing.T) {
	skip.IfCondition(t, testEnv.IsRemoteDaemon(), "cannot run daemon when remote daemon")
	t.Parallel()

	d := daemon.New(t, "", dockerdBinary, daemon.Config{})
	d.Start(t, "--iptables=false")
	defer d.Stop(t)

	client, err := d.NewClient()
	assert.Check(t, err)
	ctx := context.Background()

	createPlugin(t, client, "test", "dummy", asLogDriver)
	err = client.PluginEnable(ctx, "test", types.PluginEnableOptions{Timeout: 30})
	assert.Check(t, err)
	defer client.PluginRemove(ctx, "test", types.PluginRemoveOptions{Force: true})

	d.Stop(t)
	d.Start(t, "--iptables=false", "--log-driver=test", "--log-opt=foo=bar")
}
