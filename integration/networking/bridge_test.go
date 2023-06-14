package networking

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

// TestBridgeICC tries to ping container ctr1 from container ctr2 using its hostname. Thus, this test checks:
// 1. DNS resolution ; 2. ARP/NDP ; 3. whether containers can communicate with each other.
func TestBridgeICC(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	d := daemon.New(t)
	d.StartWithBusybox(t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name       string
		pingCmd    []string
		bridgeOpts []func(create *types.NetworkCreate)
		linkLocal  bool
		skipMsg    string
	}{
		{
			name:       "IPv4 non-internal network",
			pingCmd:    []string{"ping", "-c1", "-W3"},
			bridgeOpts: []func(create *types.NetworkCreate){},
		},
		{
			name:    "IPv4 internal network",
			pingCmd: []string{"ping", "-c1", "-W3"},
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithInternal(),
			},
		},
		{
			name:    "IPv6 ULA on non-internal network",
			pingCmd: []string{"ping", "-c1", "-W3", "-6"},
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
			},
		},
		{
			name:    "IPv6 ULA on internal network",
			pingCmd: []string{"ping", "-c1", "-W3", "-6"},
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithIPv6(),
				network.WithInternal(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			skipMsg: "See moby/moby#45649",
		},
		{
			name:    "IPv6 link-local address on non-internal network",
			pingCmd: []string{"ping", "-c1", "-W3", "-6"},
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithIPv6(),
				// There's no real way to specify an IPv6 network is only used with SLAAC link-local IPv6 addresses.
				// What we can do instead, is to tell the IPAM driver to assign addresses from the link-local prefix.
				// Each container will have two link-local addresses: 1. a SLAAC address assigned by the kernel ;
				// 2. the one dynamically assigned by the IPAM driver.
				network.WithIPAM("fe80::/64", "fe80::1"),
			},
			linkLocal: true,
		},
		{
			name:    "IPv6 link-local address on internal network",
			pingCmd: []string{"ping", "-c1", "-W3", "-6"},
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithIPv6(),
				network.WithInternal(),
				// See the note above about link-local addresses.
				network.WithIPAM("fe80::/64", "fe80::1"),
			},
			linkLocal: true,
			skipMsg:   "See moby/moby#45649",
		},
	}

	for tcID, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			skip.If(t, tc.skipMsg != "", tc.skipMsg)
			ctx := context.Background()

			bridgeName := fmt.Sprintf("testnet-icc-%d", tcID)
			network.CreateNoError(ctx, t, c, bridgeName, append(tc.bridgeOpts,
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridgeName))...)
			defer network.RemoveNoError(ctx, t, c, bridgeName)

			ctr1Name := fmt.Sprintf("ctr-icc-%d-1", tcID)
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("/bin/sleep", "infinity"),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, id1, types.ContainerRemoveOptions{
				Force: true,
			})

			pingHost := ctr1Name
			if tc.linkLocal {
				inspect := container.Inspect(ctx, t, c, id1)
				pingHost = inspect.NetworkSettings.Networks[bridgeName].GlobalIPv6Address + "%eth0"
			}

			ctr2Name := fmt.Sprintf("ctr-icc-%d-2", tcID)
			id2 := container.Run(ctx, t, c,
				container.WithName(ctr2Name),
				container.WithImage("busybox:latest"),
				container.WithCmd(append(tc.pingCmd, pingHost)...),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, id2, types.ContainerRemoveOptions{
				Force: true,
			})

			exitCode, err := d.ContainerExitCode(t, id2)
			assert.NilError(t, err)
			assert.Check(t, exitCode == int64(0))

			logs := getContainerLogs(t, ctx, c, id2)
			assert.Check(t, strings.Contains(logs, "1 packets transmitted, 1 packets received"))

			if t.Failed() {
				t.Logf("Logs from %s:\n%s", ctr2Name, logs)
			}
		})
	}
}

// TestBridgeIPv6SLAACLLAddress is pretty much the same as TestBridgeICC, but this one checks whether SLAAC addresses
// assigned by the kernel work properly. To do so, the MAC address of ctr1 is manually assigned.
func TestBridgeIPv6SLAACLLAddress(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	d := daemon.New(t)
	d.StartWithBusybox(t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name       string
		bridgeOpts []func(create *types.NetworkCreate)
		skipMsg    string
	}{
		{
			name:       "IPv6 non-internal network",
			bridgeOpts: []func(create *types.NetworkCreate){},
		},
		{
			name: "IPv6 internal network",
			bridgeOpts: []func(create *types.NetworkCreate){
				network.WithInternal(),
			},
			skipMsg: "See moby/moby#45649",
		},
	}

	for tcID, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			skip.If(t, tc.skipMsg != "", tc.skipMsg)
			ctx := context.Background()

			bridgeName := fmt.Sprintf("testnet-ip6ll-%d", tcID)
			network.CreateNoError(ctx, t, c, bridgeName, append(tc.bridgeOpts,
				network.WithDriver("bridge"),
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
				network.WithOption("com.docker.network.bridge.name", bridgeName))...)
			defer network.RemoveNoError(ctx, t, c, bridgeName)

			ctr1Name := sanitizeCtrName(t.Name() + "-ctr1")
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("/bin/sleep", "infinity"),
				// Link-local address is derived from the MAC address, so we need to
				// specify one here to hardcode the SLAAC LL address pinged below.
				container.WithMacAddress("02:42:ac:11:00:02"),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, id1, types.ContainerRemoveOptions{
				Force: true,
			})

			ctr2Name := sanitizeCtrName(t.Name() + "-ctr2")
			id2 := container.Run(ctx, t, c,
				container.WithName(ctr2Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("ping", "-c1", "-W3", "-6", "fe80::42:acff:fe11:2%eth0"),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, id2, types.ContainerRemoveOptions{
				Force: true,
			})

			exitCode, err := d.ContainerExitCode(t, id2)
			assert.NilError(t, err)
			assert.Check(t, exitCode == int64(0))

			logs := getContainerLogs(t, ctx, c, id2)
			assert.Check(t, strings.Contains(logs, "1 packets transmitted, 1 packets received"))

			if t.Failed() {
				t.Logf("Logs from %s:\n%s", ctr2Name, logs)
			}
		})
	}
}

// TestBridgeINC makes sure two containers on two different bridge networks can't communicate with each other.
func TestBridgeINC(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	d := daemon.New(t)
	d.StartWithBusybox(t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const bridge1Opts = "bridge1"
	const bridge2Opts = "bridge2"

	testcases := []struct {
		name    string
		pingCmd []string
		bridges map[string][]func(*types.NetworkCreate)
		ipv6    bool
	}{
		{
			name:    "IPv4 non-internal network",
			pingCmd: []string{"ping", "-c1", "-W3"},
			bridges: map[string][]func(*types.NetworkCreate){
				bridge1Opts: {},
				bridge2Opts: {},
			},
		},
		{
			name:    "IPv4 internal network",
			pingCmd: []string{"ping", "-c1", "-W3"},
			bridges: map[string][]func(*types.NetworkCreate){
				bridge1Opts: {network.WithInternal()},
				bridge2Opts: {network.WithInternal()},
			},
		},
		{
			name:    "IPv6 ULA on non-internal network",
			pingCmd: []string{"ping", "-c1", "-W3"},
			bridges: map[string][]func(*types.NetworkCreate){
				bridge1Opts: {
					network.WithIPv6(),
					network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
				},
				bridge2Opts: {
					network.WithIPv6(),
					network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
				},
			},
			ipv6: true,
		},
		{
			name:    "IPv6 ULA on internal network",
			pingCmd: []string{"ping", "-c1", "-W3"},
			bridges: map[string][]func(*types.NetworkCreate){
				bridge1Opts: {
					network.WithIPv6(),
					network.WithInternal(),
					network.WithIPAM("fdf1:a844:390c:b200::/64", "fdf1:a844:390c:b200::1"),
				},
				bridge2Opts: {
					network.WithIPv6(),
					network.WithInternal(),
					network.WithIPAM("fdf1:a844:390c:b247::/64", "fdf1:a844:390c:b247::1"),
				},
			},
			ipv6: true,
		},
	}

	for tcID, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			bridge1 := fmt.Sprintf("testnet-inc-%d-1", tcID)
			bridge2 := fmt.Sprintf("testnet-inc-%d-2", tcID)

			network.CreateNoError(ctx, t, c, bridge1, append(tc.bridges[bridge1Opts],
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridge1))...)
			defer network.RemoveNoError(ctx, t, c, bridge1)
			network.CreateNoError(ctx, t, c, bridge2, append(tc.bridges[bridge2Opts],
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridge2))...)
			defer network.RemoveNoError(ctx, t, c, bridge2)

			ctr1Name := sanitizeCtrName(t.Name() + "-ctr1")
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("/bin/sleep", "infinity"),
				container.WithNetworkMode(bridge1))
			defer c.ContainerRemove(ctx, id1, types.ContainerRemoveOptions{
				Force: true,
			})

			ctr1Info := container.Inspect(ctx, t, c, id1)
			targetAddr := ctr1Info.NetworkSettings.Networks[bridge1].IPAddress
			if tc.ipv6 {
				targetAddr = ctr1Info.NetworkSettings.Networks[bridge1].GlobalIPv6Address
			}

			t.Logf("Ping command: %+v", append(tc.pingCmd, targetAddr))

			ctr2Name := sanitizeCtrName(t.Name() + "-ctr2")
			id2 := container.Run(ctx, t, c,
				container.WithName(ctr2Name),
				container.WithImage("busybox:latest"),
				container.WithCmd(append(tc.pingCmd, targetAddr)...),
				container.WithNetworkMode(bridge2))
			defer c.ContainerRemove(ctx, id2, types.ContainerRemoveOptions{
				Force: true,
			})

			exitCode, err := d.ContainerExitCode(t, id2)
			assert.NilError(t, err)
			assert.Check(t, exitCode == int64(1), "ping unexpectedly succeeded")

			logs := getContainerLogs(t, ctx, c, id2)
			assert.Check(t, strings.Contains(logs, "1 packets transmitted, 0 packets received"))

			if t.Failed() {
				t.Logf("Logs from %s:\n%s", ctr2Name, logs)
			}
		})
	}
}
