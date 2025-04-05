package bridge

import (
	"context"
	"fmt"
	"net/netip"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/versions"
	ctr "github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

func TestCreateWithMultiNetworks(t *testing.T) {
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

func TestCreateWithIPv6DefaultsToULAPrefix(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const nwName = "testnetula"
	network.CreateNoError(ctx, t, apiClient, nwName, network.WithIPv6())
	defer network.RemoveNoError(ctx, t, apiClient, nwName)

	nw, err := apiClient.NetworkInspect(ctx, "testnetula", networktypes.InspectOptions{})
	assert.NilError(t, err)

	for _, ipam := range nw.IPAM.Config {
		ipr := netip.MustParsePrefix(ipam.Subnet)
		if netip.MustParsePrefix("fd00::/8").Overlaps(ipr) {
			return
		}
	}

	t.Fatalf("Network %s has no ULA prefix, expected one.", nwName)
}

func TestCreateWithIPv6WithoutEnableIPv6Flag(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--default-network-opt=bridge=com.docker.network.enable_ipv6=true")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	const nwName = "testnetula"
	network.CreateNoError(ctx, t, apiClient, nwName)
	defer network.RemoveNoError(ctx, t, apiClient, nwName)

	nw, err := apiClient.NetworkInspect(ctx, "testnetula", networktypes.InspectOptions{})
	assert.NilError(t, err)

	for _, ipam := range nw.IPAM.Config {
		ipr := netip.MustParsePrefix(ipam.Subnet)
		if netip.MustParsePrefix("fd00::/8").Overlaps(ipr) {
			return
		}
	}

	t.Fatalf("Network %s has no ULA prefix, expected one.", nwName)
}

// Check that it's possible to create IPv6 networks with a 64-bit ip-range,
// in 64-bit and bigger subnets, with and without a gateway.
func Test64BitIPRange(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	type kv struct{ k, v string }
	subnets := []kv{
		{"64-bit-subnet", "fd2e:b68c:ce26::/64"},
		{"56-bit-subnet", "fd2e:b68c:ce26::/56"},
	}
	ipRanges := []kv{
		{"no-range", ""},
		{"64-bit-range", "fd2e:b68c:ce26::/64"},
	}
	gateways := []kv{
		{"no-gateway", ""},
		{"with-gateway", "fd2e:b68c:ce26::1"},
	}

	for _, sn := range subnets {
		for _, ipr := range ipRanges {
			for _, gw := range gateways {
				ipamSetter := network.WithIPAMRange(sn.v, ipr.v, gw.v)
				t.Run(sn.k+"/"+ipr.k+"/"+gw.k, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					const netName = "test64br"
					network.CreateNoError(ctx, t, c, netName, network.WithIPv6(), ipamSetter)
					defer network.RemoveNoError(ctx, t, c, netName)
				})
			}
		}
	}
}

// Demonstrate a limitation of the IP address allocator, it can't
// allocate the last address in range that ends on a 64-bit boundary.
func TestIPRangeAt64BitLimit(t *testing.T) {
	ctx := setupTest(t)
	c := testEnv.APIClient()

	tests := []struct {
		name    string
		subnet  string
		ipRange string
	}{
		{
			name:    "ipRange before end of 64-bit subnet",
			subnet:  "fda9:8d04:086e::/64",
			ipRange: "fda9:8d04:086e::ffff:ffff:ffff:ff0e/127",
		},
		{
			name:    "ipRange at end of 64-bit subnet",
			subnet:  "fda9:8d04:086e::/64",
			ipRange: "fda9:8d04:086e::ffff:ffff:ffff:fffe/127",
		},
		{
			name:    "ipRange at 64-bit boundary inside 56-bit subnet",
			subnet:  "fda9:8d04:086e::/56",
			ipRange: "fda9:8d04:086e:aa:ffff:ffff:ffff:fffe/127",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			const netName = "test64bl"
			network.CreateNoError(ctx, t, c, netName,
				network.WithIPv6(),
				network.WithIPAMRange(tc.subnet, tc.ipRange, ""),
			)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := ctr.Create(ctx, t, c, ctr.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
			err := c.ContainerStart(ctx, id, containertypes.StartOptions{})
			assert.NilError(t, err)
		})
	}
}

// TestFilterForwardPolicy tests that, if the daemon enables IP forwarding on the
// host, it also sets the iptables filter-FORWARD policy to DROP (unless it's
// told not to).
func TestFilterForwardPolicy(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootless has its own netns")
	skip.If(t, networking.FirewalldRunning(), "can't use firewalld in host netns to add rules in L3Segment")
	ctx := setupTest(t)

	// Set up a netns for each test to avoid sysctl and iptables pollution.
	addr4 := netip.MustParseAddr("192.168.125.1")
	addr6 := netip.MustParseAddr("fd76:c828:41f9::1")
	l3 := networking.NewL3Segment(t, "test-ffp",
		netip.PrefixFrom(addr4, 24),
		netip.PrefixFrom(addr6, 64),
	)
	t.Cleanup(func() { l3.Destroy(t) })

	tests := []struct {
		name           string
		initForwarding string
		daemonArgs     []string
		expForwarding  string
		expPolicy      string
	}{
		{
			name:           "enable forwarding",
			initForwarding: "0",
			expForwarding:  "1",
			expPolicy:      "DROP",
		},
		{
			name:           "forwarding already enabled",
			initForwarding: "1",
			expForwarding:  "1",
			expPolicy:      "ACCEPT",
		},
		{
			name:           "no drop",
			initForwarding: "0",
			daemonArgs:     []string{"--ip-forward-no-drop"},
			expForwarding:  "1",
			expPolicy:      "ACCEPT",
		},
		{
			name:           "no forwarding",
			initForwarding: "0",
			daemonArgs:     []string{"--ip-forward=false"},
			expForwarding:  "0",
			expPolicy:      "ACCEPT",
		},
	}

	for i, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			// Create a netns for this test.
			addr4, addr6 = addr4.Next(), addr6.Next()
			hostname := fmt.Sprintf("docker%d", i)
			l3.AddHost(t, hostname, hostname+"-host", "eth0",
				netip.PrefixFrom(addr4, 24),
				netip.PrefixFrom(addr6, 64),
			)
			host := l3.Hosts[hostname]

			getFwdPolicy := func(cmd string) string {
				t.Helper()
				out := host.MustRun(t, cmd, "-S", "FORWARD")
				if strings.HasPrefix(out, "-P FORWARD ACCEPT") {
					return "ACCEPT"
				}
				if strings.HasPrefix(out, "-P FORWARD DROP") {
					return "DROP"
				}
				t.Fatalf("Failed to determine %s FORWARD policy: %s", cmd, out)
				return ""
			}

			type sysctls struct{ v4, v6def, v6all string }
			getSysctls := func() sysctls {
				t.Helper()
				return sysctls{
					host.MustRun(t, "sysctl", "-n", "net.ipv4.ip_forward")[:1],
					host.MustRun(t, "sysctl", "-n", "net.ipv6.conf.default.forwarding")[:1],
					host.MustRun(t, "sysctl", "-n", "net.ipv6.conf.all.forwarding")[:1],
				}
			}

			// Initial settings for IP forwarding params.
			host.MustRun(t, "sysctl", "-w", "net.ipv4.ip_forward="+tc.initForwarding)
			host.MustRun(t, "sysctl", "-w", "net.ipv6.conf.all.forwarding="+tc.initForwarding)

			// Start the daemon in its own network namespace.
			var d *daemon.Daemon
			host.Do(t, func() {
				// Run without OTel because there's no routing from this netns for it - which
				// means the daemon doesn't shut down cleanly, causing the test to fail.
				d = daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
				d.StartWithBusybox(ctx, t, tc.daemonArgs...)
				t.Cleanup(func() { d.Stop(t) })
			})
			c := d.NewClientT(t)
			t.Cleanup(func() { c.Close() })

			// If necessary, the IPv4 policy should have been updated when the default bridge network was created.
			assert.Check(t, is.Equal(getFwdPolicy("iptables"), tc.expPolicy))
			// IPv6 policy should not have been updated yet.
			assert.Check(t, is.Equal(getFwdPolicy("ip6tables"), "ACCEPT"))
			assert.Check(t, is.Equal(getSysctls(), sysctls{tc.expForwarding, tc.initForwarding, tc.initForwarding}))

			// If necessary, creating an IPv6 network should update the sysctls and policy.
			const netName = "testnetffp"
			network.CreateNoError(ctx, t, c, netName, network.WithIPv6())
			t.Cleanup(func() { network.RemoveNoError(ctx, t, c, netName) })
			assert.Check(t, is.Equal(getFwdPolicy("iptables"), tc.expPolicy))
			assert.Check(t, is.Equal(getFwdPolicy("ip6tables"), tc.expPolicy))
			assert.Check(t, is.Equal(getSysctls(), sysctls{tc.expForwarding, tc.expForwarding, tc.expForwarding}))
		})
	}
}

// TestPointToPoint checks that a "/31" --internal network with inhibit_ipv4,
// or gateway mode "isolated" has two addresses available for containers (no
// address is reserved for a gateway, because it won't be used).
func TestPointToPoint(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	testcases := []struct {
		name   string
		netOpt func(*networktypes.CreateOptions)
	}{
		{
			name:   "inhibit_ipv4",
			netOpt: network.WithOption(bridge.InhibitIPv4, "true"),
		},
		{
			name:   "isolated",
			netOpt: network.WithOption(bridge.IPv4GatewayMode, "isolated"),
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			const netName = "testp2pbridge"
			network.CreateNoError(ctx, t, apiClient, netName,
				network.WithIPAM("192.168.135.0/31", ""),
				network.WithInternal(),
				tc.netOpt,
			)
			defer network.RemoveNoError(ctx, t, apiClient, netName)

			const ctrName = "ctr1"
			id := ctr.Run(ctx, t, apiClient,
				ctr.WithNetworkMode(netName),
				ctr.WithName(ctrName),
			)
			defer apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := ctr.RunAttach(attachCtx, t, apiClient,
				ctr.WithCmd([]string{"ping", "-c1", "-W3", ctrName}...),
				ctr.WithNetworkMode(netName),
			)
			defer apiClient.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})
			assert.Check(t, is.Equal(res.ExitCode, 0))
			assert.Check(t, is.Equal(res.Stderr.Len(), 0))
			assert.Check(t, is.Contains(res.Stdout.String(), "1 packets transmitted, 1 packets received"))
		})
	}
}

// TestIsolated tests an internal network with gateway mode "isolated".
func TestIsolated(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't inspect bridge addrs in rootless netns")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const netName = "testisol"
	const bridgeName = "br-" + netName
	network.CreateNoError(ctx, t, apiClient, netName,
		network.WithIPv6(),
		network.WithInternal(),
		network.WithOption(bridge.IPv4GatewayMode, "isolated"),
		network.WithOption(bridge.IPv6GatewayMode, "isolated"),
		network.WithOption(bridge.BridgeName, bridgeName),
	)
	defer network.RemoveNoError(ctx, t, apiClient, netName)

	// The bridge should not have any IP addresses.
	link, err := nlwrap.LinkByName(bridgeName)
	assert.NilError(t, err)
	addrs, err := nlwrap.AddrList(link, netlink.FAMILY_ALL)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(len(addrs), 0))

	const ctrName = "ctr1"
	id := ctr.Run(ctx, t, apiClient,
		ctr.WithNetworkMode(netName),
		ctr.WithName(ctrName),
	)
	defer apiClient.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

	ping := func(t *testing.T, ipv string) {
		t.Helper()
		attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		res := ctr.RunAttach(attachCtx, t, apiClient,
			ctr.WithCmd([]string{"ping", "-c1", "-W3", ipv, "ctr1"}...),
			ctr.WithNetworkMode(netName),
		)
		defer apiClient.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})
		if ipv == "-6" && networking.FirewalldRunning() {
			// FIXME(robmry) - this fails due to https://github.com/moby/moby/issues/49680
			assert.Check(t, is.Equal(res.ExitCode, 1))
			t.Skip("XFAIL - IPv6, firewalld, isolated - see https://github.com/moby/moby/issues/49680")
		}
		assert.Check(t, is.Equal(res.ExitCode, 0))
		assert.Check(t, is.Equal(res.Stderr.Len(), 0))
		assert.Check(t, is.Contains(res.Stdout.String(), "1 packets transmitted, 1 packets received"))
	}
	t.Run("ipv4", func(t *testing.T) { ping(t, "-4") })
	t.Run("ipv6", func(t *testing.T) { ping(t, "-6") })
}

func TestEndpointWithCustomIfname(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	ctrID := ctr.Run(ctx, t, apiClient,
		ctr.WithCmd("ip", "-o", "link", "show", "foobar"),
		ctr.WithEndpointSettings("bridge", &networktypes.EndpointSettings{
			DriverOpts: map[string]string{
				netlabel.Ifname: "foobar",
			},
		}))
	defer ctr.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	out, err := ctr.Output(ctx, apiClient, ctrID)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(out.Stdout, ": foobar@if"), "expected ': foobar@if' in 'ip link show':\n%s", out.Stdout)
}

// TestPublishedPortAlreadyInUse checks that a container that can't start
// because of one its published port being already in use doesn't end up
// triggering the restart loop.
//
// Regression test for: https://github.com/moby/moby/issues/49501
func TestPublishedPortAlreadyInUse(t *testing.T) {
	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	ctr1 := ctr.Run(ctx, t, apiClient,
		ctr.WithCmd("top"),
		ctr.WithExposedPorts("80/tcp"),
		ctr.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8000"}}}))
	defer ctr.Remove(ctx, t, apiClient, ctr1, containertypes.RemoveOptions{Force: true})

	ctr2 := ctr.Create(ctx, t, apiClient,
		ctr.WithCmd("top"),
		ctr.WithRestartPolicy(containertypes.RestartPolicyAlways),
		ctr.WithExposedPorts("80/tcp"),
		ctr.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8000"}}}))
	defer ctr.Remove(ctx, t, apiClient, ctr2, containertypes.RemoveOptions{Force: true})

	err := apiClient.ContainerStart(ctx, ctr2, containertypes.StartOptions{})
	assert.Assert(t, is.ErrorContains(err, "failed to set up container networking"))

	inspect, err := apiClient.ContainerInspect(ctx, ctr2)
	assert.NilError(t, err)
	assert.Check(t, is.Equal(inspect.State.Status, "created"))
}

// TestAllPortMappingsAreReturned check that dual-stack ports mapped through
// different networks are correctly reported as dual-stakc.
//
// Regression test for https://github.com/moby/moby/issues/49654.
func TestAllPortMappingsAreReturned(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--userland-proxy=false")
	defer d.Stop(t)

	apiClient := d.NewClientT(t)
	defer apiClient.Close()

	nwV4 := network.CreateNoError(ctx, t, apiClient, "testnetv4")
	defer network.RemoveNoError(ctx, t, apiClient, nwV4)

	nwV6 := network.CreateNoError(ctx, t, apiClient, "testnetv6",
		network.WithIPv4(false),
		network.WithIPv6())
	defer network.RemoveNoError(ctx, t, apiClient, nwV6)

	ctrID := ctr.Run(ctx, t, apiClient,
		ctr.WithExposedPorts("80/tcp", "81/tcp"),
		ctr.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8000"}}}),
		ctr.WithEndpointSettings("testnetv4", &networktypes.EndpointSettings{}),
		ctr.WithEndpointSettings("testnetv6", &networktypes.EndpointSettings{}))
	defer ctr.Remove(ctx, t, apiClient, ctrID, containertypes.RemoveOptions{Force: true})

	inspect := ctr.Inspect(ctx, t, apiClient, ctrID)
	assert.DeepEqual(t, inspect.NetworkSettings.Ports, nat.PortMap{
		"80/tcp": []nat.PortBinding{
			{HostIP: "0.0.0.0", HostPort: "8000"},
			{HostIP: "::", HostPort: "8000"},
		},
		"81/tcp": nil,
	})
}

// TestFirewalldReloadNoZombies checks that when firewalld is reloaded, rules
// belonging to deleted networks/containers do not reappear.
func TestFirewalldReloadNoZombies(t *testing.T) {
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
