package bridge

import (
	"fmt"
	"net/netip"
	"testing"

	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestNetworkInitError checks that, if the default bridge network can't be restored on startup,
// it doesn't prevent the daemon from starting once the underlying problem is resolved.
// Regression test for https://github.com/moby/moby/issues/49291
func TestNetworkInitErrorDocker0(t *testing.T) {
	d := daemon.New(t)
	d.Start(t)
	defer func() {
		_ = d.StopWithError()
	}()

	const brName = "docker0"
	d.SetEnvVar("DOCKER_TEST_BRIDGE_INIT_ERROR", brName)
	err := d.RestartWithError()
	assert.Assert(t, is.ErrorContains(err, "daemon exited during startup"))

	d.SetEnvVar("DOCKER_TEST_BRIDGE_INIT_ERROR", "")
	d.Start(t)
}

// TestNetworkInitErrorUserDefined is equivalent to TestNetworkInitErrorDocker0, for a
// user-defined network. But, the daemon doesn't try to delete a user-defined network
// and the daemon will still start if it can't be restored on startup. So, try to
// delete the network when it's failed to initialise, and check that it can be
// re-created when the initialisation problem has been resolved.
func TestNetworkInitErrorUserDefined(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.Start(t)
	defer func() {
		_ = d.StopWithError()
	}()

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "testnet"
	const brName = "br-" + netName
	network.CreateNoError(ctx, t, c, netName,
		network.WithOption(bridge.BridgeName, brName),
	)
	defer network.RemoveNoError(ctx, t, c, netName)

	d.SetEnvVar("DOCKER_TEST_BRIDGE_INIT_ERROR", brName)
	d.Restart(t)

	err := c.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	d.SetEnvVar("DOCKER_TEST_BRIDGE_INIT_ERROR", "")
	d.Restart(t)
	network.CreateNoError(ctx, t, c, netName,
		network.WithOption(bridge.BridgeName, brName),
	)
}

// TestRecreateDefaultBridgeWhenUnsettingDefaultAddrPools checks that the
// default bridge is recreated using a subnet coming from the default value of
// the "default-address-pools" parameter when it's unset.
//
// Regression test for: https://github.com/moby/moby/issues/49353
func TestRecreateDefaultBridgeWhenUnsettingDefaultAddrPools(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")

	// Run the test in a separate netns to avoid interference with leftovers
	// from other tests or manual run of the daemon (eg. in a dev container).
	defer netnsutils.SetupTestOSContext(t)()

	ctx := setupTest(t)

	var customPool = netip.MustParsePrefix("10.20.128.0/17")

	d := daemon.New(t)
	d.Start(t, fmt.Sprintf("--default-address-pool=base=%s,size=24", customPool.String()))
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	nw, err := c.NetworkInspect(ctx, "bridge", networktypes.InspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, customPool.Contains(netip.MustParsePrefix(nw.IPAM.Config[0].Subnet).Addr()), "%s not in %s", nw.IPAM.Config[0].Subnet, customPool)

	d.Restart(t)

	nw, err = c.NetworkInspect(ctx, "bridge", networktypes.InspectOptions{})
	assert.NilError(t, err)
	assert.Check(t, inDefaultAddressPools(netip.MustParsePrefix(nw.IPAM.Config[0].Subnet)))
}

// inDefaultAddressPools checks whether the given prefix is in the default
// "default-address-pools".
func inDefaultAddressPools(p netip.Prefix) bool {
	for _, addrPool := range ipamutils.GetLocalScopeDefaultNetworks() {
		if addrPool.Overlaps(p) {
			return true
		}
	}
	return false
}
