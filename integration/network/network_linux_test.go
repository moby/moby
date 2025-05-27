package network // import "github.com/docker/docker/integration/network"

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

func TestRunContainerWithBridgeNone(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.IsUserNamespace)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-b", "none")
	defer d.Stop(t)

	c := d.NewClientT(t)

	id1 := container.Run(ctx, t, c)
	defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

	result, err := container.Exec(ctx, c, id1, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in default(bridge) mode when bridge network is disabled")

	id2 := container.Run(ctx, t, c, container.WithNetworkMode("bridge"))
	defer c.ContainerRemove(ctx, id2, containertypes.RemoveOptions{Force: true})

	result, err = container.Exec(ctx, c, id2, []string{"ip", "l"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(false, strings.Contains(result.Combined(), "eth0")), "There shouldn't be eth0 in container in bridge mode when bridge network is disabled")

	nsCommand := "ls -l /proc/self/ns/net | awk -F '->' '{print $2}'"
	cmd := exec.Command("sh", "-c", nsCommand)
	stdout := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	err = cmd.Run()
	assert.NilError(t, err, "Failed to get current process network namespace: %+v", err)

	id3 := container.Run(ctx, t, c, container.WithNetworkMode("host"))
	defer c.ContainerRemove(ctx, id3, containertypes.RemoveOptions{Force: true})

	result, err = container.Exec(ctx, c, id3, []string{"sh", "-c", nsCommand})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(stdout.String(), result.Combined()), "The network namespace of container should be the same with host when --net=host and bridge network is disabled")
}

func TestHostIPv4BridgeLabel(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.Start(t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	ipv4SNATAddr := "172.0.0.172"
	// Create a bridge network with --opt com.docker.network.host_ipv4=172.0.0.172
	bridgeName := "hostIPv4Bridge"
	network.CreateNoError(ctx, t, c, bridgeName,
		network.WithDriver("bridge"),
		network.WithOption("com.docker.network.host_ipv4", ipv4SNATAddr),
		network.WithOption("com.docker.network.bridge.name", bridgeName),
	)
	defer network.RemoveNoError(ctx, t, c, bridgeName)
	out, err := c.NetworkInspect(ctx, bridgeName, networktypes.InspectOptions{Verbose: true})
	assert.NilError(t, err)
	assert.Assert(t, len(out.IPAM.Config) > 0)
	// Make sure the SNAT rule exists
	testutil.RunCommand(ctx, "iptables", "-t", "nat", "-C", "POSTROUTING", "-s", out.IPAM.Config[0].Subnet, "!", "-o", bridgeName, "-j", "SNAT", "--to-source", ipv4SNATAddr).Assert(t, icmd.Success)
}

func TestDefaultNetworkOpts(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	ctx := testutil.StartSpan(baseContext, t)

	tests := []struct {
		name       string
		mtu        int
		configFrom bool
		args       []string
	}{
		{
			name: "default value",
			mtu:  1500,
			args: []string{},
		},
		{
			name: "cmdline value",
			mtu:  1234,
			args: []string{"--default-network-opt", "bridge=com.docker.network.driver.mtu=1234"},
		},
		{
			name:       "config-from value",
			configFrom: true,
			mtu:        1233,
			args:       []string{"--default-network-opt", "bridge=com.docker.network.driver.mtu=1234"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			d := daemon.New(t)
			d.StartWithBusybox(ctx, t, tc.args...)
			defer d.Stop(t)
			c := d.NewClientT(t)
			defer c.Close()

			if tc.configFrom {
				// Create a new network config
				network.CreateNoError(ctx, t, c, "from-net", func(create *networktypes.CreateOptions) {
					create.ConfigOnly = true
					create.Options = map[string]string{
						"com.docker.network.driver.mtu": fmt.Sprint(tc.mtu),
					}
				})
				defer c.NetworkRemove(ctx, "from-net")
			}

			// Create a new network
			networkName := "testnet"
			networkId := network.CreateNoError(ctx, t, c, networkName, func(create *networktypes.CreateOptions) {
				if tc.configFrom {
					create.ConfigFrom = &networktypes.ConfigReference{
						Network: "from-net",
					}
				}
			})
			defer c.NetworkRemove(ctx, networkName)

			// Check the MTU of the bridge itself, before any devices are connected. (The
			// bridge's MTU will be set to the minimum MTU of anything connected to it, but
			// it's set explicitly on the bridge anyway - so it doesn't look like the option
			// was ignored.)
			cmd := exec.Command("ip", "link", "show", "br-"+networkId[:12])
			output, err := cmd.CombinedOutput()
			assert.NilError(t, err)
			assert.Check(t, is.Contains(string(output), fmt.Sprintf(" mtu %d ", tc.mtu)), "Bridge MTU should have been set to %d", tc.mtu)

			// Start a container to inspect the MTU of its network interface
			id1 := container.Run(ctx, t, c, container.WithNetworkMode(networkName))
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

			result, err := container.Exec(ctx, c, id1, []string{"ip", "l", "show", "eth0"})
			assert.NilError(t, err)
			assert.Check(t, is.Contains(result.Combined(), fmt.Sprintf(" mtu %d ", tc.mtu)), "Network MTU should have been set to %d", tc.mtu)
		})
	}
}

func TestForbidDuplicateNetworkNames(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	network.CreateNoError(ctx, t, c, "testnet")
	defer network.RemoveNoError(ctx, t, c, "testnet")

	_, err := c.NetworkCreate(ctx, "testnet", networktypes.CreateOptions{})
	assert.Error(t, err, "Error response from daemon: network with name testnet already exists", "2nd NetworkCreate call should have failed")
}

// TestHostGatewayFromDocker0 checks that, when docker0 has IPv6, host-gateway maps to both IPv4 and IPv6.
func TestHostGatewayFromDocker0(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)

	// Run the daemon in its own n/w namespace, to avoid interfering with
	// the docker0 bridge belonging to the daemon started by CI.
	const name = "host-gw-ips"
	l3 := networking.NewL3Segment(t, "test-"+name)
	defer l3.Destroy(t)
	l3.AddHost(t, "host-gw-ips", "host-gw-ips", "eth0")

	// Run without OTEL because there's no routing from this netns for it - which
	// means the daemon doesn't shut down cleanly, causing the test to fail.
	d := daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
	l3.Hosts[name].Do(t, func() {
		d.StartWithBusybox(ctx, t, "--ipv6",
			"--fixed-cidr", "192.168.50.0/24",
			"--fixed-cidr-v6", "fddd:6ff4:6e08::/64",
		)
	})
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	res := container.RunAttach(ctx, t, c,
		container.WithExtraHost("hg:host-gateway"),
		container.WithCmd("grep", "hg$", "/etc/hosts"),
	)
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Contains(res.Stdout.String(), "192.168.50.1\thg"))
	assert.Check(t, is.Contains(res.Stdout.String(), "fddd:6ff4:6e08::1\thg"))
}

func TestCreateWithPriority(t *testing.T) {
	// This feature should work on Windows, but the test is skipped because:
	// 1. Linux-specific tools are used here; 2. 'windows' IPAM driver doesn't
	// support static allocations.
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.48"), "requires API v1.48")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	network.CreateNoError(ctx, t, apiClient, "testnet1",
		network.WithIPv6(),
		network.WithIPAM("10.100.20.0/24", "10.100.20.1"),
		network.WithIPAM("fd54:7a1b:8269::/64", "fd54:7a1b:8269::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet1")

	network.CreateNoError(ctx, t, apiClient, "testnet2",
		network.WithIPv6(),
		network.WithIPAM("10.100.30.0/24", "10.100.30.1"),
		network.WithIPAM("fdff:6dfe:37d2::/64", "fdff:6dfe:37d2::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet2")

	ctrID := container.Run(ctx, t, apiClient,
		container.WithCmd("sleep", "infinity"),
		container.WithNetworkMode("testnet1"),
		container.WithEndpointSettings("testnet1", &networktypes.EndpointSettings{GwPriority: 10}),
		container.WithEndpointSettings("testnet2", &networktypes.EndpointSettings{GwPriority: 100}))
	defer container.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 3, "default via 10.100.30.1 dev")
	// IPv6 routing table will contain for each interface, one route for the LL
	// address, one for the ULA, and one multicast.
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 7, "default via fdff:6dfe:37d2::1 dev")
}

func TestConnectWithPriority(t *testing.T) {
	// This feature should work on Windows, but the test is skipped because:
	// 1. Linux-specific tools are used here; 2. 'windows' IPAM driver doesn't
	// support static allocations.
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.48"), "requires API v1.48")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	network.CreateNoError(ctx, t, apiClient, "testnet1",
		network.WithIPv6(),
		network.WithIPAM("10.100.10.0/24", "10.100.10.1"),
		network.WithIPAM("fddd:4901:f594::/64", "fddd:4901:f594::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet1")

	network.CreateNoError(ctx, t, apiClient, "testnet2",
		network.WithIPv6(),
		network.WithIPAM("10.100.20.0/24", "10.100.20.1"),
		network.WithIPAM("fd83:7683:7008::/64", "fd83:7683:7008::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet2")

	network.CreateNoError(ctx, t, apiClient, "testnet3",
		network.WithDriver("bridge"),
		network.WithIPv6(),
		network.WithIPAM("10.100.30.0/24", "10.100.30.1"),
		network.WithIPAM("fd72:de0:adad::/64", "fd72:de0:adad::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet3")

	network.CreateNoError(ctx, t, apiClient, "testnet4",
		network.WithIPv6(),
		network.WithIPAM("10.100.40.0/24", "10.100.40.1"),
		network.WithIPAM("fd4c:c927:7d90::/64", "fd4c:c927:7d90::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet4")

	network.CreateNoError(ctx, t, apiClient, "testnet5",
		network.WithIPv6(),
		network.WithIPAM("10.100.50.0/24", "10.100.50.1"),
		network.WithIPAM("fd4c:364b:1110::/64", "fd4c:364b:1110::1"))
	defer network.RemoveNoError(ctx, t, apiClient, "testnet5")

	ctrID := container.Run(ctx, t, apiClient,
		container.WithCmd("sleep", "infinity"),
		container.WithNetworkMode("testnet1"),
		container.WithEndpointSettings("testnet1", &networktypes.EndpointSettings{}))
	defer container.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 2, "default via 10.100.10.1 dev eth0")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 4, "default via fddd:4901:f594::1 dev eth0")

	// testnet5 has a negative priority -- the default gateway should not change.
	err := apiClient.NetworkConnect(ctx, "testnet5", ctrID, &networktypes.EndpointSettings{GwPriority: -100})
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 3, "default via 10.100.10.1 dev eth0")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 7, "default via fddd:4901:f594::1 dev eth0")

	// testnet2 has a higher priority. It should now provide the default gateway.
	err = apiClient.NetworkConnect(ctx, "testnet2", ctrID, &networktypes.EndpointSettings{GwPriority: 100})
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 4, "default via 10.100.20.1 dev eth2")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 10, "default via fd83:7683:7008::1 dev eth2")

	// testnet3 has a lower priority, so testnet2 should still provide the default gateway.
	err = apiClient.NetworkConnect(ctx, "testnet3", ctrID, &networktypes.EndpointSettings{GwPriority: 10})
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 5, "default via 10.100.20.1 dev eth2")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 13, "default via fd83:7683:7008::1 dev eth2")

	// testnet4 has the same priority as testnet3, but it sorts after in
	// lexicographic order. For now, testnet2 stays the default gateway.
	err = apiClient.NetworkConnect(ctx, "testnet4", ctrID, &networktypes.EndpointSettings{GwPriority: 10})
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 6, "default via 10.100.20.1 dev eth2")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 16, "default via fd83:7683:7008::1 dev eth2")

	inspect := container.Inspect(ctx, t, apiClient, ctrID)
	assert.Equal(t, inspect.NetworkSettings.Networks["testnet1"].GwPriority, 0)
	assert.Equal(t, inspect.NetworkSettings.Networks["testnet2"].GwPriority, 100)
	assert.Equal(t, inspect.NetworkSettings.Networks["testnet3"].GwPriority, 10)
	assert.Equal(t, inspect.NetworkSettings.Networks["testnet4"].GwPriority, 10)
	assert.Equal(t, inspect.NetworkSettings.Networks["testnet5"].GwPriority, -100)

	// Disconnect testnet2, so testnet3 should now provide the default gateway.
	// When two endpoints have the same priority (eg. testnet3 vs testnet4),
	// the one that sorts first in lexicographic order is picked.
	err = apiClient.NetworkDisconnect(ctx, "testnet2", ctrID, true)
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 5, "default via 10.100.30.1 dev eth3")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 13, "default via fd72:de0:adad::1 dev eth3")

	// Disconnect testnet3, so testnet4 should now provide the default gateway.
	err = apiClient.NetworkDisconnect(ctx, "testnet3", ctrID, true)
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 4, "default via 10.100.40.1 dev eth4")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 10, "default via fd4c:c927:7d90::1 dev eth4")

	// Disconnect testnet4, so testnet1 should now provide the default gateway.
	err = apiClient.NetworkDisconnect(ctx, "testnet4", ctrID, true)
	assert.NilError(t, err)
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET, 3, "default via 10.100.10.1 dev eth0")
	checkCtrRoutes(t, ctx, apiClient, ctrID, syscall.AF_INET6, 7, "default via fddd:4901:f594::1 dev eth0")
}

// checkCtrRoutes execute 'ip route show' in a container, and check that the
// number of routes matches expRoutes. It also checks that the default route
// matches expDefRoute. A substring match is used to avoid issues with
// non-stable interface names.
func checkCtrRoutes(t *testing.T, ctx context.Context, apiClient client.APIClient, ctrID string, af, expRoutes int, expDefRoute string) {
	t.Helper()

	fam := "-4"
	if af == syscall.AF_INET6 {
		fam = "-6"
	}

	ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()
	res, err := container.Exec(ctx, apiClient, ctrID, []string{"ip", "-o", fam, "route", "show"})
	assert.NilError(t, err)

	assert.Equal(t, res.ExitCode, 0)
	assert.Equal(t, res.Stderr(), "")

	routes := slices.DeleteFunc(strings.Split(res.Stdout(), "\n"), func(s string) bool {
		return s == ""
	})

	assert.Check(t, is.Equal(len(routes), expRoutes), "expected %d routes, got %d:\n%s", expRoutes, len(routes), strings.Join(routes, "\n"))
	if expDefRoute == "" {
		defFound := slices.ContainsFunc(routes, func(s string) bool {
			return strings.HasPrefix(s, "default")
		})
		assert.Check(t, !defFound, "unexpected default route\n%s", strings.Join(routes, "\n"))
	} else {
		defFound := slices.ContainsFunc(routes, func(s string) bool {
			return strings.Contains(s, expDefRoute)
		})
		assert.Check(t, defFound, "default route %q not found:\n%s", expDefRoute, strings.Join(routes, "\n"))
	}
}

// TestMixL3IPVlanAndBridge checks that a container can be connected to a layer-3
// ipvlan network as well as a bridge ... the bridge network will set up a
// default gateway, if selected as the gateway endpoint. The ipvlan driver sets
// up a connected route to 0.0.0.0 or [::], a route via a specific interface with
// no next-hop address (because the next-hop address can't be ARP'd to determine
// the interface). These two types of route cannot be set up at the same time.
// So, the ipvlan's route must be treated like the default gateway and only get
// set up when the ipvlan is selected as the gateway endpoint.
// Regression test for https://github.com/moby/moby/issues/48576
func TestMixL3IPVlanAndBridge(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "no ipvlan on Windows")
	skip.If(t, versions.LessThan(testEnv.DaemonAPIVersion(), "1.48"), "gw-priority requires API v1.48")
	skip.If(t, testEnv.IsRootless, "can't see the dummy parent interface from the rootless namespace")

	ctx := testutil.StartSpan(baseContext, t)

	tests := []struct {
		name        string
		liveRestore bool
	}{
		{
			name: "no live restore",
		},
		{
			// If the daemon is restarted with a running container, the osSbox structure
			// must be repopulated correctly in order for gateways to be removed then
			// re-added when network connections change.
			name:        "live restore",
			liveRestore: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			d := daemon.New(t)
			var daemonArgs []string
			if tc.liveRestore {
				daemonArgs = append(daemonArgs, "--live-restore")
			}
			d.StartWithBusybox(ctx, t, daemonArgs...)
			defer d.Stop(t)
			c := d.NewClientT(t)
			defer c.Close()

			const br46NetName = "br46net"
			network.CreateNoError(ctx, t, c, br46NetName,
				network.WithOption(netlabel.ContainerIfacePrefix, "bds"),
				network.WithIPv6(),
				network.WithIPAM("192.168.123.0/24", "192.168.123.1"),
				network.WithIPAM("fd6f:36f8:3005::/64", "fd6f:36f8:3005::1"),
			)
			defer network.RemoveNoError(ctx, t, c, br46NetName)

			const br6NetName = "br6net"
			network.CreateNoError(ctx, t, c, br6NetName,
				network.WithOption(netlabel.ContainerIfacePrefix, "bss"),
				network.WithIPv4(false),
				network.WithIPv6(),
				network.WithIPAM("fdc9:adaf:b5da::/64", "fdc9:adaf:b5da::1"),
			)
			defer network.RemoveNoError(ctx, t, c, br6NetName)

			// Create a dummy parent interface rather than letting the driver do it because,
			// when the driver creates its own, it becomes a '--internal' network and no
			// default route is configured.
			const parentIfName = "di-dummy0"
			CreateMasterDummy(ctx, t, parentIfName)
			defer DeleteInterface(ctx, t, parentIfName)

			const ipvNetName = "ipvnet"
			network.CreateNoError(ctx, t, c, ipvNetName,
				network.WithDriver("ipvlan"),
				network.WithOption("ipvlan_mode", "l3"),
				network.WithOption("parent", parentIfName),
				network.WithIPv6(),
				network.WithIPAM("192.168.124.0/24", ""),
				network.WithIPAM("fd7d:8755:51ba::/64", ""),
			)
			defer network.RemoveNoError(ctx, t, c, ipvNetName)

			// Create a container connected to all three networks, bridge network acting as gateway.
			ctrId := container.Run(ctx, t, c,
				container.WithNetworkMode(br46NetName),
				container.WithEndpointSettings(br46NetName,
					&networktypes.EndpointSettings{GwPriority: 1},
				),
				container.WithEndpointSettings(br6NetName, &networktypes.EndpointSettings{}),
				container.WithEndpointSettings(ipvNetName, &networktypes.EndpointSettings{}),
			)
			defer container.Remove(ctx, t, c, ctrId, containertypes.RemoveOptions{Force: true})

			if tc.liveRestore {
				d.Restart(t, daemonArgs...)
			}

			// Expect three IPv4 routes: the default, plus one per network.
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET, 3, "default via 192.168.123.1 dev bds")
			// Expect ten IPv6 routes: the default, plus UL, LL, and multicast routes per network.
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET6, 10, "default via fd6f:36f8:3005::1 dev bds")

			// Disconnect the dual-stack bridge network, expect the ipvlan's default route to be set up.
			c.NetworkDisconnect(ctx, br46NetName, ctrId, false)
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET, 2, "default dev eth")
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET6, 7, "default dev eth")

			// Disconnect the ipvlan, expect the IPv6-only network to be the gateway, with no IPv4 gateway.
			// (For this to work in the live-restore case the "dstName" of the interface must have been
			// restored in the osSbox, based on matching the running interface's IPv6 address.)
			c.NetworkDisconnect(ctx, ipvNetName, ctrId, false)
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET, 0, "")
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET6, 4, "default via fdc9:adaf:b5da::1 dev bss")

			// Reconnect the dual-stack bridge, expect it to be the gateway for both addr families.
			c.NetworkConnect(ctx, br46NetName, ctrId, &networktypes.EndpointSettings{GwPriority: 1})
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET, 2, "default via 192.168.123.1 dev bds")
			checkCtrRoutes(t, ctx, c, ctrId, syscall.AF_INET6, 7, "default via fd6f:36f8:3005::1 dev bds")
		})
	}
}
