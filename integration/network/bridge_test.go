package network

import (
	"context"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	ctr "github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

func TestCreateWithMultiNetworks(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.44"), "requires API v1.44")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	network.CreateNoError(ctx, t, apiClient, "testnet1")
	defer network.RemoveNoError(ctx, t, apiClient, "testnet1")

	network.CreateNoError(ctx, t, apiClient, "testnet2")
	defer network.RemoveNoError(ctx, t, apiClient, "testnet2")

	attachCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	res := ctr.RunAttach(attachCtx, t, apiClient,
		ctr.WithCmd("ip", "-o", "-4", "addr", "show"),
		ctr.WithNetworkMode("testnet1"),
		ctr.WithEndpointSettings("testnet1", &networktypes.EndpointSettings{}),
		ctr.WithEndpointSettings("testnet2", &networktypes.EndpointSettings{}))

	assert.Equal(t, res.ExitCode, 0)
	assert.Equal(t, res.Stderr.String(), "")

	// Only interfaces with an IPv4 address are printed by iproute2 when flag -4 is specified. Here, we should have two
	// interfaces for testnet1 and testnet2, plus lo.
	ifacesWithAddress := strings.Count(res.Stdout.String(), "\n")
	assert.Equal(t, ifacesWithAddress, 3)
}

// TestFirewalldReloadNoZombies checks that when firewalld is reloaded, rules
// belonging to deleted networks/containers do not reappear.
func TestFirewalldReloadNoZombies(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, !networking.FirewalldRunning(), "firewalld is not running")
	skip.If(t, testEnv.IsRootless, "no firewalld in rootless netns")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)

	const bridgeName = "br-fwdreload"
	removed := false
	nw := network.CreateNoError(ctx, t, c, "testnet",
		network.WithOption(bridge.BridgeName, bridgeName))
	defer func() {
		if !removed {
			network.RemoveNoError(ctx, t, c, nw)
		}
	}()

	cid := ctr.Run(ctx, t, c,
		ctr.WithExposedPorts("80/tcp", "81/tcp"),
		ctr.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8000"}}}))
	defer func() {
		if !removed {
			ctr.Remove(ctx, t, c, cid, containertypes.RemoveOptions{Force: true})
		}
	}()

	iptablesSave := icmd.Command("iptables-save")
	resBeforeDel := icmd.RunCmd(iptablesSave)
	assert.NilError(t, resBeforeDel.Error)
	assert.Check(t, strings.Contains(resBeforeDel.Combined(), bridgeName),
		"With container: expected rules for %s in: %s", bridgeName, resBeforeDel.Combined())

	// Delete the container and its network.
	ctr.Remove(ctx, t, c, cid, containertypes.RemoveOptions{Force: true})
	network.RemoveNoError(ctx, t, c, nw)
	removed = true

	// Check the network does not appear in iptables rules.
	resAfterDel := icmd.RunCmd(iptablesSave)
	assert.NilError(t, resAfterDel.Error)
	assert.Check(t, !strings.Contains(resAfterDel.Combined(), bridgeName),
		"After deletes: did not expect rules for %s in: %s", bridgeName, resAfterDel.Combined())

	// firewall-cmd --reload, and wait for the daemon to restore rules.
	networking.FirewalldReload(t, d)

	// Check that rules for the deleted container/network have not reappeared.
	resAfterReload := icmd.RunCmd(iptablesSave)
	assert.NilError(t, resAfterReload.Error)
	assert.Check(t, !strings.Contains(resAfterReload.Combined(), bridgeName),
		"After deletes: did not expect rules for %s in: %s", bridgeName, resAfterReload.Combined())
}
