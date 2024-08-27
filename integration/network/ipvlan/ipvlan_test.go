//go:build !windows

package ipvlan // import "github.com/docker/docker/integration/network/ipvlan"

import (
	"context"
	"fmt"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	dclient "github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	n "github.com/docker/docker/integration/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkIpvlanPersistence(t *testing.T) {
	// verify the driver automatically provisions the 802.1q link (di-dummy0.70)
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	// master dummy interface 'di' notation represent 'docker ipvlan'
	master := "di-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	c := d.NewClientT(t)

	// create a network specifying the desired sub-interface name
	netName := "di-persist"
	net.CreateNoError(ctx, t, c, netName,
		net.WithIPvlan("di-dummy0.70", ""),
	)

	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName))
	// Restart docker daemon to test the config has persisted to disk
	d.Restart(t)
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName))
}

func TestDockerNetworkIpvlan(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	for _, tc := range []struct {
		name string
		test func(*testing.T, context.Context, dclient.APIClient)
	}{
		{
			name: "Subinterface",
			test: testIpvlanSubinterface,
		}, {
			name: "OverlapParent",
			test: testIpvlanOverlapParent,
		}, {
			name: "L2NilParent",
			test: testIpvlanL2NilParent,
		}, {
			name: "L2InternalMode",
			test: testIpvlanL2InternalMode,
		}, {
			name: "L3NilParent",
			test: testIpvlanL3NilParent,
		}, {
			name: "L3InternalMode",
			test: testIpvlanL3InternalMode,
		}, {
			name: "L2MultiSubnetWithParent",
			test: testIpvlanL2MultiSubnetWithParent,
		}, {
			name: "L2MultiSubnetNoParent",
			test: testIpvlanL2MultiSubnetNoParent,
		}, {
			name: "L3MultiSubnet",
			test: testIpvlanL3MultiSubnet,
		}, {
			name: "L2Addressing",
			test: testIpvlanL2Addressing,
		}, {
			name: "L3Addressing",
			test: testIpvlanL3Addressing,
		}, {
			name: "NoIPv6",
			test: testIpvlanNoIPv6,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.StartSpan(ctx, t)
			d := daemon.New(t)
			t.Cleanup(func() { d.Stop(t) })
			d.StartWithBusybox(ctx, t)
			c := d.NewClientT(t)
			tc.test(t, ctx, c)
		})

		// FIXME(vdemeester) clean network
	}
}

func testIpvlanSubinterface(t *testing.T, ctx context.Context, client dclient.APIClient) {
	master := "di-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "di-subinterface"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("di-dummy0.60", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	// delete the network while preserving the parent link
	err := client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, "di-dummy0")
}

func testIpvlanOverlapParent(t *testing.T, ctx context.Context, client dclient.APIClient) {
	// verify the same parent interface cannot be used if already in use by an existing network
	master := "di-dummy0"
	parent := master + ".30"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)
	n.CreateVlanInterface(ctx, t, master, parent, "30")

	netName := "di-subinterface"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan(parent, ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	_, err := net.Create(ctx, client, netName,
		net.WithIPvlan(parent, ""),
	)
	// verify that the overlap returns an error
	assert.Check(t, err != nil)
}

func testIpvlanL2NilParent(t *testing.T, ctx context.Context, client dclient.APIClient) {
	// ipvlan l2 mode - dummy parent interface is provisioned dynamically
	netName := "di-nil-parent"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, client, container.WithNetworkMode(netName))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL2InternalMode(t *testing.T, ctx context.Context, client dclient.APIClient) {
	netName := "di-internal"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", ""),
		net.WithInternal(),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, client, container.WithNetworkMode(netName))

	result, _ := container.Exec(ctx, client, id1, []string{"ping", "-c", "1", "8.8.8.8"})
	assert.Check(t, strings.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL3NilParent(t *testing.T, ctx context.Context, client dclient.APIClient) {
	netName := "di-nil-parent-l3"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", "l3"),
		net.WithIPAM("172.28.230.0/24", ""),
		net.WithIPAM("172.28.220.0/24", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.220.10"),
	)
	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.230.10"),
	)

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL3InternalMode(t *testing.T, ctx context.Context, client dclient.APIClient) {
	netName := "di-internal-l3"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", "l3"),
		net.WithInternal(),
		net.WithIPAM("172.28.230.0/24", ""),
		net.WithIPAM("172.28.220.0/24", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.220.10"),
	)
	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.230.10"),
	)

	result, _ := container.Exec(ctx, client, id1, []string{"ping", "-c", "1", "8.8.8.8"})
	assert.Check(t, strings.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL2MultiSubnetWithParent(t *testing.T, ctx context.Context, client dclient.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)
	testIpvlanL2MultiSubnet(t, ctx, client, parentIfName)
}

func testIpvlanL2MultiSubnetNoParent(t *testing.T, ctx context.Context, client dclient.APIClient) {
	testIpvlanL2MultiSubnet(t, ctx, client, "")
}

func testIpvlanL2MultiSubnet(t *testing.T, ctx context.Context, client dclient.APIClient, parent string) {
	netName := "dualstackl2"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan(parent, ""),
		net.WithIPv6(),
		net.WithIPAM("172.28.200.0/24", ""),
		net.WithIPAM("172.28.202.0/24", "172.28.202.254"),
		net.WithIPAM("2001:db8:abc8::/64", ""),
		net.WithIPAM("2001:db8:abc6::/64", "2001:db8:abc6::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.200.20"),
		container.WithIPv6(netName, "2001:db8:abc8::20"),
	)
	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.200.21"),
		container.WithIPv6(netName, "2001:db8:abc8::21"),
	)
	c1, err := client.ContainerInspect(ctx, id1)
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks[netName].Gateway, ""))
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks[netName].IPv6Gateway, ""))
	} else {
		// Inspect the v4 gateway to ensure the proper default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks[netName].Gateway, "172.28.200.1"))
		// Inspect the v6 gateway to ensure the proper default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks[netName].IPv6Gateway, "2001:db8:abc8::1"))
	}

	// verify ipv4 connectivity to the explicit --ip address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", c1.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping6", "-c", "1", c1.NetworkSettings.Networks[netName].GlobalIPv6Address})
	assert.NilError(t, err)

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.202.20"),
		container.WithIPv6(netName, "2001:db8:abc6::20"),
	)
	id4 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.202.21"),
		container.WithIPv6(netName, "2001:db8:abc6::21"),
	)
	c3, err := client.ContainerInspect(ctx, id3)
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks[netName].Gateway, ""))
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks[netName].IPv6Gateway, ""))
	} else {
		// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks[netName].Gateway, "172.28.202.254"))
		// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks[netName].IPv6Gateway, "2001:db8:abc6::254"))
	}

	// verify ipv4 connectivity to the explicit --ip address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping", "-c", "1", c3.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping6", "-c", "1", c3.NetworkSettings.Networks[netName].GlobalIPv6Address})
	assert.NilError(t, err)
}

func testIpvlanL3MultiSubnet(t *testing.T, ctx context.Context, client dclient.APIClient) {
	netName := "dualstackl3"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", "l3"),
		net.WithIPv6(),
		net.WithIPAM("172.28.10.0/24", ""),
		net.WithIPAM("172.28.12.0/24", "172.28.12.254"),
		net.WithIPAM("2001:db8:abc9::/64", ""),
		net.WithIPAM("2001:db8:abc7::/64", "2001:db8:abc7::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.10.20"),
		container.WithIPv6(netName, "2001:db8:abc9::20"),
	)
	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.10.21"),
		container.WithIPv6(netName, "2001:db8:abc9::21"),
	)
	c1, err := client.ContainerInspect(ctx, id1)
	assert.NilError(t, err)

	// verify ipv4 connectivity to the explicit --ipv address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", c1.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping6", "-c", "1", c1.NetworkSettings.Networks[netName].GlobalIPv6Address})
	assert.NilError(t, err)

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.12.20"),
		container.WithIPv6(netName, "2001:db8:abc7::20"),
	)
	id4 := container.Run(ctx, t, client,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.12.21"),
		container.WithIPv6(netName, "2001:db8:abc7::21"),
	)
	c3, err := client.ContainerInspect(ctx, id3)
	assert.NilError(t, err)

	// verify ipv4 connectivity to the explicit --ipv address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping", "-c", "1", c3.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping6", "-c", "1", c3.NetworkSettings.Networks[netName].GlobalIPv6Address})
	assert.NilError(t, err)

	// Inspect the v4 gateway to ensure no next hop is assigned in L3 mode
	assert.Equal(t, c1.NetworkSettings.Networks[netName].Gateway, "")
	// Inspect the v6 gateway to ensure the explicitly specified default GW is ignored per L3 mode enabled
	assert.Equal(t, c1.NetworkSettings.Networks[netName].IPv6Gateway, "")
	// Inspect the v4 gateway to ensure no next hop is assigned in L3 mode
	assert.Equal(t, c3.NetworkSettings.Networks[netName].Gateway, "")
	// Inspect the v6 gateway to ensure the explicitly specified default GW is ignored per L3 mode enabled
	assert.Equal(t, c3.NetworkSettings.Networks[netName].IPv6Gateway, "")
}

// Verify ipvlan l2 mode sets the proper default gateway routes via netlink
// for either an explicitly set route by the user or inferred via default IPAM
func testIpvlanL2Addressing(t *testing.T, ctx context.Context, client dclient.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	netNameL2 := "dualstackl2"
	net.CreateNoError(ctx, t, client, netNameL2,
		net.WithIPvlan(parentIfName, "l2"),
		net.WithIPv6(),
		net.WithIPAM("172.28.140.0/24", "172.28.140.254"),
		net.WithIPAM("2001:db8:abcb::/64", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netNameL2))

	id := container.Run(ctx, t, client,
		container.WithNetworkMode(netNameL2),
	)
	// Validate ipvlan l2 mode defaults gateway sets the default IPAM next-hop inferred from the subnet
	result, err := container.Exec(ctx, client, id, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default via 172.28.140.254 dev eth0"))
	// Validate ipvlan l2 mode sets the v6 gateway to the user specified default gateway/next-hop
	result, err = container.Exec(ctx, client, id, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default via 2001:db8:abcb::1 dev eth0"))
}

// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
func testIpvlanL3Addressing(t *testing.T, ctx context.Context, client dclient.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	netNameL3 := "dualstackl3"
	net.CreateNoError(ctx, t, client, netNameL3,
		net.WithIPvlan(parentIfName, "l3"),
		net.WithIPv6(),
		net.WithIPAM("172.28.160.0/24", "172.28.160.254"),
		net.WithIPAM("2001:db8:abcd::/64", "2001:db8:abcd::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netNameL3))

	id := container.Run(ctx, t, client,
		container.WithNetworkMode(netNameL3),
	)
	// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err := container.Exec(ctx, client, id, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default dev eth0"))
	// Validate ipvlan l3 mode sets the v6 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err = container.Exec(ctx, client, id, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default dev eth0"))
}

// Check that an ipvlan interface with '--ipv6=false' doesn't get kernel-assigned
// IPv6 addresses, but the loopback interface does still have an IPv6 address ('::1').
func testIpvlanNoIPv6(t *testing.T, ctx context.Context, client dclient.APIClient) {
	const netName = "ipvlannet"
	net.CreateNoError(ctx, t, client, netName, net.WithIPvlan("", "l3"))
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id := container.Run(ctx, t, client, container.WithNetworkMode(netName))

	loRes := container.ExecT(ctx, t, client, id, []string{"ip", "a", "show", "dev", "lo"})
	assert.Check(t, is.Contains(loRes.Combined(), " inet "))
	assert.Check(t, is.Contains(loRes.Combined(), " inet6 "))

	eth0Res := container.ExecT(ctx, t, client, id, []string{"ip", "a", "show", "dev", "eth0"})
	assert.Check(t, is.Contains(eth0Res.Combined(), " inet "))
	assert.Check(t, !strings.Contains(eth0Res.Combined(), " inet6 "),
		"result.Combined(): %s", eth0Res.Combined())

	sysctlRes := container.ExecT(ctx, t, client, id, []string{"sysctl", "-n", "net.ipv6.conf.eth0.disable_ipv6"})
	assert.Check(t, is.Equal(strings.TrimSpace(sysctlRes.Combined()), "1"))
}

// TestIPVlanDNS checks whether DNS is forwarded, for combinations of l2/l3 mode,
// with/without a parent interface, and with '--internal'. Note that, there's no
// attempt here to give the ipvlan network external connectivity - when this test
// supplies a parent interface, it's a dummy. External DNS lookups only work
// because the daemon is configured to see a host resolver on a loopback
// interface, so the external DNS lookup happens in the host's namespace. The
// test is checking that an automatically configured dummy interface causes the
// network to behave as if it was '--internal'. Regression test for
// https://github.com/moby/moby/issues/47662
func TestIPVlanDNS(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")
	ctx := testutil.StartSpan(baseContext, t)

	net.StartDaftDNS(t, "127.0.0.1")

	tmpFileName := net.WriteTempResolvConf(t, "127.0.0.1")
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_RESOLV_CONF_PATH="+tmpFileName))
	d.StartWithBusybox(ctx, t)
	t.Cleanup(func() { d.Stop(t) })
	c := d.NewClientT(t)

	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	const netName = "ipvlan-dns-net"

	testcases := []struct {
		name     string
		parent   string
		internal bool
		expDNS   bool
	}{
		{
			name:   "with parent",
			parent: parentIfName,
			// External DNS should be used (even though the network has no external connectivity).
			expDNS: true,
		},
		{
			name: "no parent",
			// External DNS should not be used, equivalent to '--internal'.
		},
		{
			name:     "with parent, internal",
			parent:   parentIfName,
			internal: true,
			// External DNS should not be used.
		},
	}

	for _, mode := range []string{"l2", "l3"} {
		for _, tc := range testcases {
			name := fmt.Sprintf("Mode=%v/HasParent=%v/Internal=%v", mode, tc.parent != "", tc.internal)
			t.Run(name, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				createOpts := []func(*network.CreateOptions){
					net.WithIPvlan(tc.parent, mode),
				}
				if tc.internal {
					createOpts = append(createOpts, net.WithInternal())
				}
				net.CreateNoError(ctx, t, c, netName, createOpts...)
				defer c.NetworkRemove(ctx, netName)

				ctrId := container.Run(ctx, t, c, container.WithNetworkMode(netName))
				defer c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})
				res, err := container.Exec(ctx, c, ctrId, []string{"nslookup", "test.example"})
				assert.NilError(t, err)
				if tc.expDNS {
					assert.Check(t, is.Equal(res.ExitCode, 0))
					assert.Check(t, is.Contains(res.Stdout(), net.DNSRespAddr))
				} else {
					assert.Check(t, is.Equal(res.ExitCode, 1))
					assert.Check(t, is.Contains(res.Stdout(), "SERVFAIL"))
				}
			})
		}
	}
}
