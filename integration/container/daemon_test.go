package container

import (
	"testing"

	containertypes "github.com/moby/moby/api/types/container"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/testutil"
	"github.com/moby/moby/v2/testutil/daemon"
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

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Cleanup(t)

	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	// The intention of this container is to ignore stop signals.
	// Sadly this means the test will take longer, but at least this test can be parallelized.
	id := container.Run(ctx, t, apiClient, container.WithCmd("/bin/sh", "-c", "while true; do echo hello; sleep 1; done"))
	defer func() {
		err := apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
		assert.NilError(t, err)
	}()

	inspect, err := apiClient.ContainerInspect(ctx, id)
	assert.NilError(t, err)
	assert.Assert(t, inspect.State.Running)

	assert.NilError(t, d.Kill())
	d.Start(t, "--iptables=false", "--ip6tables=false")

	inspect, err = apiClient.ContainerInspect(ctx, id)
	assert.Check(t, is.Nil(err))
	assert.Assert(t, !inspect.State.Running)
}

// When the daemon doesn't stop in a clean way (eg. it crashes, the host has a power failure, etc..), or if it's started
// with live-restore enabled, stopped containers should have their NetworkSettings cleaned up the next time the daemon
// starts.
func TestNetworkStateCleanupOnDaemonStart(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless, "scenario doesn't work with rootless mode")

	t.Parallel()

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	defer d.Cleanup(t)

	d.StartWithBusybox(ctx, t, "--iptables=false", "--ip6tables=false")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)

	// The intention of this container is to ignore stop signals.
	// Sadly this means the test will take longer, but at least this test can be parallelized.
	cid := container.Run(ctx, t, apiClient,
		container.WithExposedPorts("80/tcp"),
		container.WithPortMap(map[containertypes.PortProto][]containertypes.PortBinding{"80/tcp": {{}}}),
		container.WithCmd("/bin/sh", "-c", "while true; do echo hello; sleep 1; done"))
	defer func() {
		err := apiClient.ContainerRemove(ctx, cid, containertypes.RemoveOptions{Force: true})
		assert.NilError(t, err)
	}()

	inspect, err := apiClient.ContainerInspect(ctx, cid)
	assert.NilError(t, err)
	assert.Assert(t, inspect.NetworkSettings.SandboxID != "")
	assert.Assert(t, inspect.NetworkSettings.SandboxKey != "")
	assert.Assert(t, inspect.NetworkSettings.Ports["80/tcp"] != nil)

	assert.NilError(t, d.Kill())
	d.Start(t)

	inspect, err = apiClient.ContainerInspect(ctx, cid)
	assert.NilError(t, err)
	assert.Assert(t, inspect.NetworkSettings.SandboxID == "")
	assert.Assert(t, inspect.NetworkSettings.SandboxKey == "")
	assert.Assert(t, is.Nil(inspect.NetworkSettings.Ports["80/tcp"]))
}
