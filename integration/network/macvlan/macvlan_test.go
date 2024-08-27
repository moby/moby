//go:build !windows

package macvlan // import "github.com/docker/docker/integration/network/macvlan"

import (
	"context"
	"strings"
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	n "github.com/docker/docker/integration/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkMacvlanPersistence(t *testing.T) {
	// verify the driver automatically provisions the 802.1q link (dm-dummy0.60)
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	c := d.NewClientT(t)

	netName := "dm-persist"
	net.CreateNoError(ctx, t, c, netName,
		net.WithMacvlan("dm-dummy0.60"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName))
	d.Restart(t)
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName))
}

func TestDockerNetworkMacvlan(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	for _, tc := range []struct {
		name string
		test func(*testing.T, context.Context, client.APIClient)
	}{
		{
			name: "Subinterface",
			test: testMacvlanSubinterface,
		}, {
			name: "OverlapParent",
			test: testMacvlanOverlapParent,
		}, {
			name: "OverlapParentPassthruFirst",
			test: testMacvlanOverlapParentPassthruFirst,
		}, {
			name: "OverlapParentPassthruSecond",
			test: testMacvlanOverlapParentPassthruSecond,
		}, {
			name: "OverlapDeleteCreatedSecond",
			test: testMacvlanOverlapDeleteCreatedSecond,
		}, {
			name: "OverlapKeepExistingParent",
			test: testMacvlanOverlapKeepExisting,
		}, {
			name: "NilParent",
			test: testMacvlanNilParent,
		}, {
			name: "InternalMode",
			test: testMacvlanInternalMode,
		}, {
			name: "MultiSubnetWithParent",
			test: testMacvlanMultiSubnetWithParent,
		}, {
			name: "MultiSubnetNoParent",
			test: testMacvlanMultiSubnetNoParent,
		}, {
			name: "Addressing",
			test: testMacvlanAddressing,
		}, {
			name: "NoIPv6",
			test: testMacvlanNoIPv6,
		},
	} {
		tc := tc
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

func testMacvlanOverlapParent(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify the same parent interface can be used if already in use by an existing network
	// as long as neither are passthru
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))
	n.LinkExists(ctx, t, parentName)

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, client, overlapNetName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err == nil)

	// delete the second network while preserving the parent link
	err = client.NetworkRemove(ctx, overlapNetName)
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, overlapNetName))
	n.LinkExists(ctx, t, parentName)

	// delete the first network
	err = client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	n.LinkDoesntExist(ctx, t, parentName)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapParentPassthruFirst(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify creating a second interface sharing a parent with another passthru interface is rejected
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlanPassthru(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	_, err := net.Create(ctx, client, "dm-parent-net-overlap",
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err != nil)

	// delete the network while preserving the parent link
	err = client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapParentPassthruSecond(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify creating a passthru interface sharing a parent with another interface is rejected
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	_, err := net.Create(ctx, client, "dm-parent-net-overlap",
		net.WithMacvlanPassthru(parentName),
	)
	assert.Check(t, err != nil)

	// delete the network while preserving the parent link
	err = client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapDeleteCreatedSecond(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify that a shared created parent interface is kept when the original interface is deleted first
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, client, overlapNetName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err == nil)

	// delete the original network while preserving the parent link
	err = client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	n.LinkExists(ctx, t, parentName)

	// delete the second network
	err = client.NetworkRemove(ctx, overlapNetName)
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, overlapNetName))
	n.LinkDoesntExist(ctx, t, parentName)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapKeepExisting(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify that deleting interfaces sharing a previously existing parent doesn't delete the
	// parent
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(master),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, client, overlapNetName,
		net.WithMacvlan(master),
	)
	assert.Check(t, err == nil)

	err = client.NetworkRemove(ctx, overlapNetName)
	assert.NilError(t, err)
	err = client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanSubinterface(t *testing.T, ctx context.Context, client client.APIClient) {
	// verify the same parent interface cannot be used if already in use by an existing network
	master := "dm-dummy0"
	parentName := "dm-dummy0.20"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)
	n.CreateVlanInterface(ctx, t, master, parentName, "20")

	netName := "dm-subinterface"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	// delete the network while preserving the parent link
	err := client.NetworkRemove(ctx, netName)
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, client, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, parentName)
}

func testMacvlanNilParent(t *testing.T, ctx context.Context, client client.APIClient) {
	// macvlan bridge mode - dummy parent interface is provisioned dynamically
	netName := "dm-nil-parent"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, client, container.WithNetworkMode(netName))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.Check(t, err == nil)
}

func testMacvlanInternalMode(t *testing.T, ctx context.Context, client client.APIClient) {
	// macvlan bridge mode - dummy parent interface is provisioned dynamically
	netName := "dm-internal"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(""),
		net.WithInternal(),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, client, container.WithNetworkMode(netName))

	result, _ := container.Exec(ctx, client, id1, []string{"ping", "-c", "1", "8.8.8.8"})
	assert.Check(t, strings.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.Check(t, err == nil)
}

func testMacvlanMultiSubnetWithParent(t *testing.T, ctx context.Context, client client.APIClient) {
	const parentIfName = "dm-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)
	testMacvlanMultiSubnet(t, ctx, client, parentIfName)
}

func testMacvlanMultiSubnetNoParent(t *testing.T, ctx context.Context, client client.APIClient) {
	testMacvlanMultiSubnet(t, ctx, client, "")
}

func testMacvlanMultiSubnet(t *testing.T, ctx context.Context, client client.APIClient, parent string) {
	netName := "dualstackbridge"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parent),
		net.WithIPv6(),
		net.WithIPAM("172.28.100.0/24", ""),
		net.WithIPAM("172.28.102.0/24", "172.28.102.254"),
		net.WithIPAM("2001:db8:abc2::/64", ""),
		net.WithIPAM("2001:db8:abc4::/64", "2001:db8:abc4::254"),
	)

	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.100.20"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc2::20"),
	)
	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.100.21"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc2::21"),
	)
	c1, err := client.ContainerInspect(ctx, id1)
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks["dualstackbridge"].Gateway, ""))
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, ""))
	} else {
		// Inspect the v4 gateway to ensure the proper default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks["dualstackbridge"].Gateway, "172.28.100.1"))
		// Inspect the v6 gateway to ensure the proper default GW was assigned
		assert.Check(t, is.Equal(c1.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, "2001:db8:abc2::1"))
	}

	// verify ipv4 connectivity to the explicit --ip address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", c1.NetworkSettings.Networks["dualstackbridge"].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping6", "-c", "1", c1.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address})
	assert.NilError(t, err)

	// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, client,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.102.20"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc4::20"),
	)
	id4 := container.Run(ctx, t, client,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.102.21"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc4::21"),
	)
	c3, err := client.ContainerInspect(ctx, id3)
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks["dualstackbridge"].Gateway, ""))
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, ""))
	} else {
		// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks["dualstackbridge"].Gateway, "172.28.102.254"))
		// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, "2001:db8:abc4::254"))
	}

	// verify ipv4 connectivity to the explicit --ip address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping", "-c", "1", c3.NetworkSettings.Networks["dualstackbridge"].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping6", "-c", "1", c3.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address})
	assert.NilError(t, err)
}

func testMacvlanAddressing(t *testing.T, ctx context.Context, client client.APIClient) {
	const parentIfName = "dm-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	// Ensure the default gateways, next-hops and default dev devices are properly set
	netName := "dualstackbridge"
	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(parentIfName),
		net.WithIPv6(),
		net.WithOption("macvlan_mode", "bridge"),
		net.WithIPAM("172.28.130.0/24", ""),
		net.WithIPAM("2001:db8:abca::/64", "2001:db8:abca::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netName))

	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode("dualstackbridge"),
	)

	// Validate macvlan bridge mode defaults gateway sets the default IPAM next-hop inferred from the subnet
	result, err := container.Exec(ctx, client, id1, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default via 172.28.130.1 dev eth0"))
	// Validate macvlan bridge mode sets the v6 gateway to the user specified default gateway/next-hop
	result, err = container.Exec(ctx, client, id1, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default via 2001:db8:abca::254 dev eth0"))
}

// Check that a macvlan interface with '--ipv6=false' doesn't get kernel-assigned
// IPv6 addresses, but the loopback interface does still have an IPv6 address ('::1').
func testMacvlanNoIPv6(t *testing.T, ctx context.Context, client client.APIClient) {
	const netName = "macvlannet"

	net.CreateNoError(ctx, t, client, netName,
		net.WithMacvlan(""),
		net.WithOption("macvlan_mode", "bridge"),
	)
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

// TestMACVlanDNS checks whether DNS is forwarded, with/without a parent
// interface, and with '--internal'. Note that there's no attempt here to give
// the macvlan network external connectivity - when this test supplies a parent
// interface, it's a dummy. External DNS lookups only work because the daemon is
// configured to see a host resolver on a loopback interface, so the external DNS
// lookup happens in the host's namespace. The test is checking that an
// automatically configured dummy interface causes the network to behave as if it
// was '--internal'.
func TestMACVlanDNS(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)

	net.StartDaftDNS(t, "127.0.0.1")

	tmpFileName := net.WriteTempResolvConf(t, "127.0.0.1")
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_RESOLV_CONF_PATH="+tmpFileName))
	d.StartWithBusybox(ctx, t)
	t.Cleanup(func() { d.Stop(t) })
	c := d.NewClientT(t)

	const parentIfName = "dm-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	const netName = "macvlan-dns-net"

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
			expDNS:   false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			createOpts := []func(*network.CreateOptions){
				net.WithMacvlan(tc.parent),
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
