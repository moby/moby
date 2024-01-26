package networking

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestMACAddrOnRestart is a regression test for https://github.com/moby/moby/issues/47146
//   - Start a container, let it use a generated MAC address.
//   - Stop that container.
//   - Start a second container, it'll also use a generated MAC address.
//     (It's likely to recycle the first container's MAC address.)
//   - Restart the first container.
//     (The bug was that it kept its original MAC address, now already in-use.)
//   - Check that the two containers have different MAC addresses.
func TestMACAddrOnRestart(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "testmacaddrs"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, netName))
	defer network.RemoveNoError(ctx, t, c, netName)

	const ctr1Name = "ctr1"
	id1 := container.Run(ctx, t, c,
		container.WithName(ctr1Name),
		container.WithImage("busybox:latest"),
		container.WithCmd("top"),
		container.WithNetworkMode(netName))
	defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{
		Force: true,
	})
	err := c.ContainerStop(ctx, ctr1Name, containertypes.StopOptions{})
	assert.Assert(t, is.Nil(err))

	// Start a second container, giving the daemon a chance to recycle the first container's
	// IP and MAC addresses.
	const ctr2Name = "ctr2"
	id2 := container.Run(ctx, t, c,
		container.WithName(ctr2Name),
		container.WithImage("busybox:latest"),
		container.WithCmd("top"),
		container.WithNetworkMode(netName))
	defer c.ContainerRemove(ctx, id2, containertypes.RemoveOptions{
		Force: true,
	})

	// Restart the first container.
	err = c.ContainerStart(ctx, ctr1Name, containertypes.StartOptions{})
	assert.Assert(t, is.Nil(err))

	// Check that the containers ended up with different MAC addresses.

	ctr1Inspect := container.Inspect(ctx, t, c, ctr1Name)
	ctr1MAC := ctr1Inspect.NetworkSettings.Networks[netName].MacAddress

	ctr2Inspect := container.Inspect(ctx, t, c, ctr2Name)
	ctr2MAC := ctr2Inspect.NetworkSettings.Networks[netName].MacAddress

	assert.Check(t, ctr1MAC != ctr2MAC,
		"expected containers to have different MAC addresses; got %q for both", ctr1MAC)
}

// Check that a configured MAC address is restored after a container restart,
// and after a daemon restart.
func TestCfgdMACAddrOnRestart(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "testcfgmacaddr"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, netName))
	defer network.RemoveNoError(ctx, t, c, netName)

	const wantMAC = "02:42:ac:11:00:42"
	const ctr1Name = "ctr1"
	id1 := container.Run(ctx, t, c,
		container.WithName(ctr1Name),
		container.WithImage("busybox:latest"),
		container.WithCmd("top"),
		container.WithNetworkMode(netName),
		container.WithMacAddress(netName, wantMAC))
	defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{
		Force: true,
	})

	inspect := container.Inspect(ctx, t, c, ctr1Name)
	gotMAC := inspect.NetworkSettings.Networks[netName].MacAddress
	assert.Check(t, is.Equal(wantMAC, gotMAC))

	startAndCheck := func() {
		t.Helper()
		err := c.ContainerStart(ctx, ctr1Name, containertypes.StartOptions{})
		assert.Assert(t, is.Nil(err))
		inspect = container.Inspect(ctx, t, c, ctr1Name)
		gotMAC = inspect.NetworkSettings.Networks[netName].MacAddress
		assert.Check(t, is.Equal(wantMAC, gotMAC))
	}

	// Restart the container, check that the MAC address is restored.
	err := c.ContainerStop(ctx, ctr1Name, containertypes.StopOptions{})
	assert.Assert(t, is.Nil(err))
	startAndCheck()

	// Restart the daemon, check that the MAC address is restored.
	err = c.ContainerStop(ctx, ctr1Name, containertypes.StopOptions{})
	assert.Assert(t, is.Nil(err))
	d.Restart(t)
	startAndCheck()
}
