package container

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// Make sure a container that does not exit when it upon receiving it's stop signal is actually shutdown on daemon
// startup.
func TestContainerKillOnDaemonStart(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "scenario doesn't work with rootless mode")

	t.Parallel()

	d := daemon.New(t)
	defer d.Cleanup(t)

	d.StartWithBusybox(t, "--iptables=false")
	defer d.Stop(t)

	client := d.NewClientT(t)
	ctx := context.Background()

	// The intention of this container is to ignore stop signals.
	// Sadly this means the test will take longer, but at least this test can be parallelized.
	id := container.Run(ctx, t, client, container.WithCmd("/bin/sh", "-c", "while true; do echo hello; sleep 1; done"))
	defer func() {
		err := client.ContainerRemove(ctx, id, types.ContainerRemoveOptions{Force: true})
		assert.NilError(t, err)
	}()

	inspect, err := client.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	assert.Assert(t, inspect.State.Running)

	assert.NilError(t, d.Kill())
	d.Start(t, "--iptables=false")

	inspect, err = client.ContainerInspect(ctx, id)
	assert.Check(t, is.Nil(err))
	assert.Assert(t, !inspect.State.Running)
}
