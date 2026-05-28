//go:build !windows

package macvlan

import (
	"context"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/libnetwork/netlabel"
	"github.com/moby/moby/v2/integration/internal/container"
	net "github.com/moby/moby/v2/integration/internal/network"
	n "github.com/moby/moby/v2/integration/network"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
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

func testMacvlanOverlapParent(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify the same parent interface can be used if already in use by an existing network
	// as long as neither are passthru
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))
	n.LinkExists(ctx, t, parentName)

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, apiClient, overlapNetName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err)

	// delete the second network while preserving the parent link
	_, err = apiClient.NetworkRemove(ctx, overlapNetName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, overlapNetName))
	n.LinkExists(ctx, t, parentName)

	// delete the first network
	_, err = apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
	n.LinkDoesntExist(ctx, t, parentName)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapParentPassthruFirst(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify creating a second interface sharing a parent with another passthru interface is rejected
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlanPassthru(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	_, err := net.Create(ctx, apiClient, "dm-parent-net-overlap",
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err != nil)

	// delete the network while preserving the parent link
	_, err = apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapParentPassthruSecond(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify creating a passthru interface sharing a parent with another interface is rejected
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	_, err := net.Create(ctx, apiClient, "dm-parent-net-overlap",
		net.WithMacvlanPassthru(parentName),
	)
	assert.Check(t, err != nil)

	// delete the network while preserving the parent link
	_, err = apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapDeleteCreatedSecond(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify that a shared created parent interface is kept when the original interface is deleted first
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	parentName := "dm-dummy0.40"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, apiClient, overlapNetName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, err)

	// delete the original network while preserving the parent link
	_, err = apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
	n.LinkExists(ctx, t, parentName)

	// delete the second network
	_, err = apiClient.NetworkRemove(ctx, overlapNetName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, overlapNetName))
	n.LinkDoesntExist(ctx, t, parentName)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanOverlapKeepExisting(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify that deleting interfaces sharing a previously existing parent doesn't delete the
	// parent
	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "dm-subinterface"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(master),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	overlapNetName := "dm-parent-net-overlap"
	_, err := net.Create(ctx, apiClient, overlapNetName,
		net.WithMacvlan(master),
	)
	assert.Check(t, err)

	_, err = apiClient.NetworkRemove(ctx, overlapNetName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)
	_, err = apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)

	// verify the network delete did not delete the root link
	n.LinkExists(ctx, t, master)
}

func testMacvlanSubinterface(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// verify the same parent interface cannot be used if already in use by an existing network
	master := "dm-dummy0"
	parentName := "dm-dummy0.20"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)
	n.CreateVlanInterface(ctx, t, master, parentName, "20")

	netName := "dm-subinterface"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(parentName),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	// delete the network while preserving the parent link
	_, err := apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
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
	assert.Check(t, err)
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
	assert.Check(t, is.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
	assert.Check(t, err)
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

func testMacvlanMultiSubnet(t *testing.T, ctx context.Context, apiClient client.APIClient, parent string) {
	netName := "dualstackbridge"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(parent),
		net.WithIPv6(),
		net.WithIPAM("172.28.100.0/24", ""),
		net.WithIPAM("172.28.102.0/24", "172.28.102.254"),
		net.WithIPAM("2001:db8:abc2::/64", ""),
		net.WithIPAM("2001:db8:abc4::/64", "2001:db8:abc4::254"),
	)

	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.100.20"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc2::20"),
	)
	id2 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.100.21"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc2::21"),
	)
	c1, err := apiClient.ContainerInspect(ctx, id1, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	// Inspect the v4 gateway to ensure no default GW was assigned
	assert.Check(t, !c1.Container.NetworkSettings.Networks["dualstackbridge"].Gateway.IsValid())
	// Inspect the v6 gateway to ensure no default GW was assigned
	assert.Check(t, !c1.Container.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway.IsValid())

	// verify ipv4 connectivity to the explicit --ip address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", c1.Container.NetworkSettings.Networks["dualstackbridge"].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping6", "-c", "1", c1.Container.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address.String()})
	assert.NilError(t, err)

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.102.20"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc4::20"),
	)
	id4 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode("dualstackbridge"),
		container.WithIPv4("dualstackbridge", "172.28.102.21"),
		container.WithIPv6("dualstackbridge", "2001:db8:abc4::21"),
	)
	c3, err := apiClient.ContainerInspect(ctx, id3, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, !c3.Container.NetworkSettings.Networks["dualstackbridge"].Gateway.IsValid())
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, !c3.Container.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway.IsValid())
	} else {
		// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.Container.NetworkSettings.Networks["dualstackbridge"].Gateway, netip.MustParseAddr("172.28.102.254")))
		// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.Container.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, netip.MustParseAddr("2001:db8:abc4::254")))
	}

	// verify ipv4 connectivity to the explicit --ip address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping", "-c", "1", c3.Container.NetworkSettings.Networks["dualstackbridge"].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping6", "-c", "1", c3.Container.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address.String()})
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

	// No gateway address was supplied for IPv4, check that no default gateway was set up.
	result, err := container.Exec(ctx, client, id1, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, !strings.Contains(result.Combined(), "default via"),
		"result: %s", result.Combined())
	// Validate macvlan bridge mode sets the v6 gateway to the user-specified default gateway/next-hop
	result, err = container.Exec(ctx, client, id1, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default via 2001:db8:abca::254 dev eth0"))
}

// Check that a macvlan interface with '--ipv6=false' doesn't get kernel-assigned
// IPv6 addresses, but the loopback interface does still have an IPv6 address ('::1').
// Also check that with '--ipv4=false', there's no IPAM-assigned IPv4 address.
func TestMacvlanIPAM(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	testcases := []struct {
		name       string
		apiVersion string
		enableIPv4 bool
		enableIPv6 bool
		expIPv4    bool
	}{
		{
			name:       "dual stack",
			enableIPv4: true,
			enableIPv6: true,
		},
		{
			name:       "v4 only",
			enableIPv4: true,
		},
		{
			name:       "v6 only",
			enableIPv6: true,
		},
		{
			name: "no ipam",
		},
		{
			name:       "enableIPv4 ignored",
			apiVersion: "1.46",
			enableIPv4: false,
			expIPv4:    true,
		},
	}
	var (
		subnetv4 = netip.MustParsePrefix("10.66.77.0/24")
		subnetv6 = netip.MustParsePrefix("2001:db8:abcd::/64")
	)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			c := d.NewClientT(t, client.WithAPIVersion(tc.apiVersion))

			netOpts := []func(*client.NetworkCreateOptions){
				net.WithMacvlan(""),
				net.WithOption("macvlan_mode", "bridge"),
				net.WithIPv4(tc.enableIPv4),
				net.WithIPAMConfig(
					network.IPAMConfig{
						Subnet:  subnetv4,
						IPRange: netip.MustParsePrefix("10.66.77.64/30"),
						Gateway: netip.MustParseAddr("10.66.77.1"),
						AuxAddress: map[string]netip.Addr{
							"inrange":    netip.MustParseAddr("10.66.77.65"),
							"outofrange": netip.MustParseAddr("10.66.77.128"),
						},
					},
					network.IPAMConfig{
						Subnet:  subnetv6,
						IPRange: netip.MustParsePrefix("2001:db8:abcd::/120"),
					},
				),
			}
			if tc.enableIPv6 {
				netOpts = append(netOpts, net.WithIPv6())
			}

			const netName = "macvlannet"
			net.CreateNoError(ctx, t, c, netName, netOpts...)
			defer c.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
			assert.Check(t, n.IsNetworkAvailable(ctx, c, netName))

			id := container.Run(ctx, t, c, container.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

			loRes := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "lo"})
			assert.Check(t, is.Contains(loRes.Combined(), " inet "))
			assert.Check(t, is.Contains(loRes.Combined(), " inet6 "))

			wantSubnetStatus := make(map[netip.Prefix]network.SubnetStatus)
			eth0Res := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "eth0"})
			if tc.enableIPv4 || tc.expIPv4 {
				assert.Check(t, is.Contains(eth0Res.Combined(), " inet "),
					"Expected IPv4 in: %s", eth0Res.Combined())
				wantSubnetStatus[subnetv4] = network.SubnetStatus{
					IPsInUse:            6, // network, gateway, 2x aux, broadcast, container
					DynamicIPsAvailable: 2, // container, aux "inrange"
				}
			} else {
				assert.Check(t, !strings.Contains(eth0Res.Combined(), " inet "),
					"Expected no IPv4 in: %s", eth0Res.Combined())
			}
			if tc.enableIPv6 {
				assert.Check(t, is.Contains(eth0Res.Combined(), " inet6 "),
					"Expected IPv6 in: %s", eth0Res.Combined())
				wantSubnetStatus[subnetv6] = network.SubnetStatus{
					IPsInUse:            2, // subnet-router anycast, container
					DynamicIPsAvailable: 254,
				}
			} else {
				assert.Check(t, !strings.Contains(eth0Res.Combined(), " inet6 "),
					"Expected no IPv6 in: %s", eth0Res.Combined())
			}

			sysctlRes := container.ExecT(ctx, t, c, id, []string{"sysctl", "-n", "net.ipv6.conf.eth0.disable_ipv6"})
			expDisableIPv6 := "1"
			if tc.enableIPv6 {
				expDisableIPv6 = "0"
			}
			assert.Check(t, is.Equal(strings.TrimSpace(sysctlRes.Combined()), expDisableIPv6))

			cc := d.NewClientT(t, client.WithAPIVersion("1.52"))
			res, err := cc.NetworkInspect(ctx, netName, client.NetworkInspectOptions{})
			if assert.Check(t, err) && assert.Check(t, res.Network.Status != nil) {
				assert.Check(t, is.DeepEqual(wantSubnetStatus, res.Network.Status.IPAM.Subnets, cmpopts.EquateEmpty()))
			}
			_ = cc.Close()
			cc = d.NewClientT(t, client.WithAPIVersion("1.51"))
			res, err = cc.NetworkInspect(ctx, netName, client.NetworkInspectOptions{})
			assert.Check(t, err)
			assert.Check(t, res.Network.Status == nil)
			_ = cc.Close()
		})
	}
}

// MACVLAN networks are allowed to be assigned IPAM subnets that overlap with
// other MACVLAN networks' IPAM subnets. But no two MACVLAN endpoints may be
// assigned the same address, even when the endpoints are attached to different
// networks. The assignment of an address to an endpoint on one network may
// therefore reduce the number of available addresses to assign to other
// networks' endpoints.
func TestMacvlanIPAMOverlap(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)

	checkNetworkIPAMState := func(networkID string, want map[netip.Prefix]network.SubnetStatus) bool {
		t.Helper()
		res, err := c.NetworkInspect(ctx, networkID, client.NetworkInspectOptions{})
		if assert.Check(t, err) && assert.Check(t, res.Network.Status != nil) {
			return assert.Check(t, is.DeepEqual(want, res.Network.Status.IPAM.Subnets, cmpopts.EquateEmpty()))
		}
		return false
	}

	// Create three networks with joined and overlapped IPAM ranges
	// and verify that the IPAM state is correct

	const (
		netName1 = "macvlannet1"
		netName2 = "macvlannet2"
		netName3 = "macvlannet3"
	)
	cidrv4 := netip.MustParsePrefix("192.168.0.0/24")
	cidrv6 := netip.MustParsePrefix("2001:db8:abcd::/64")

	net.CreateNoError(ctx, t, c, netName1,
		net.WithMacvlan(""),
		net.WithIPv6(),
		net.WithIPAMConfig(
			network.IPAMConfig{
				Subnet:  cidrv4,
				IPRange: netip.MustParsePrefix("192.168.0.0/25"),
				Gateway: netip.MustParseAddr("192.168.0.1"),
				AuxAddress: map[string]netip.Addr{
					"reserved": netip.MustParseAddr("192.168.0.100"),
				},
			},
			network.IPAMConfig{
				Subnet:  cidrv6,
				IPRange: netip.MustParsePrefix("2001:db8:abcd::/124"),
			},
		),
	)
	defer c.NetworkRemove(ctx, netName1, client.NetworkRemoveOptions{})
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName1))

	checkNetworkIPAMState(netName1, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            4,
			DynamicIPsAvailable: 125,
		},
		cidrv6: {
			IPsInUse:            1,
			DynamicIPsAvailable: 15,
		},
	})

	net.CreateNoError(ctx, t, c, netName2,
		net.WithMacvlan(""),
		net.WithIPv6(),
		net.WithIPAMConfig(
			network.IPAMConfig{
				Subnet:  cidrv4,
				IPRange: netip.MustParsePrefix("192.168.0.0/24"),
			},
			network.IPAMConfig{
				Subnet:  cidrv6,
				IPRange: netip.MustParsePrefix("2001:db8:abcd::/120"),
			},
		),
	)

	defer c.NetworkRemove(ctx, netName2, client.NetworkRemoveOptions{})
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName2))

	checkNetworkIPAMState(netName2, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            4,
			DynamicIPsAvailable: 252,
		},
		cidrv6: {
			IPsInUse:            1,
			DynamicIPsAvailable: 255,
		},
	})

	net.CreateNoError(ctx, t, c, netName3,
		net.WithMacvlan(""),
		net.WithIPv6(),
		net.WithIPAMConfig(
			network.IPAMConfig{
				Subnet:  cidrv4,
				IPRange: netip.MustParsePrefix("192.168.0.128/25"),
			},
			network.IPAMConfig{
				Subnet:  cidrv6,
				IPRange: netip.MustParsePrefix("2001:db8:abcd::80/124"),
			},
		),
	)

	defer c.NetworkRemove(ctx, netName3, client.NetworkRemoveOptions{})
	assert.Check(t, n.IsNetworkAvailable(ctx, c, netName3))

	checkNetworkIPAMState(netName3, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            4,
			DynamicIPsAvailable: 127,
		},
		cidrv6: {
			IPsInUse:            1,
			DynamicIPsAvailable: 16,
		},
	})

	// Create a container on one of the networks
	id := container.Run(ctx, t, c, container.WithNetworkMode(netName1))
	defer c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

	// Verify that the IPAM status of all three networks are affected.
	checkNetworkIPAMState(netName1, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            5,
			DynamicIPsAvailable: 124,
		},
		cidrv6: {
			IPsInUse:            2,
			DynamicIPsAvailable: 14,
		},
	})

	checkNetworkIPAMState(netName2, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            5,
			DynamicIPsAvailable: 251,
		},
		cidrv6: {
			IPsInUse:            2,
			DynamicIPsAvailable: 254,
		},
	})

	checkNetworkIPAMState(netName3, map[netip.Prefix]network.SubnetStatus{
		cidrv4: {
			IPsInUse:            5,
			DynamicIPsAvailable: 127,
		},
		cidrv6: {
			IPsInUse:            2,
			DynamicIPsAvailable: 16,
		},
	})
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

	d := daemon.New(t, daemon.WithResolvConf(net.GenResolvConf("127.0.0.1")))
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
			createOpts := []func(*client.NetworkCreateOptions){
				net.WithMacvlan(tc.parent),
			}
			if tc.internal {
				createOpts = append(createOpts, net.WithInternal())
			}
			net.CreateNoError(ctx, t, c, netName, createOpts...)
			defer c.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})

			ctrId := container.Run(ctx, t, c, container.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})
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

// TestPointToPoint checks that no gateway is reserved for a macvlan network
// with no parent interface (an "internal" network).
func TestPointToPoint(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)
	apiClient := testEnv.APIClient()

	const netName = "p2pmacvlan"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(""),
		net.WithIPAM("192.168.135.0/31", ""),
	)
	defer net.RemoveNoError(ctx, t, apiClient, netName)

	const ctrName = "ctr1"
	id := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithName(ctrName),
	)
	defer apiClient.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

	attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := container.RunAttach(attachCtx, t, apiClient,
		container.WithCmd([]string{"ping", "-c1", "-W3", ctrName}...),
		container.WithNetworkMode(netName),
	)
	defer apiClient.ContainerRemove(ctx, res.ContainerID, client.ContainerRemoveOptions{Force: true})
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Equal(res.Stderr.Len(), 0))
	assert.Check(t, is.Contains(res.Stdout.String(), "1 packets transmitted, 1 packets received"))
}

func TestEndpointWithCustomIfname(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const master = "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	// create a network specifying the desired sub-interface name
	const netName = "macvlan-custom-ifname"
	net.CreateNoError(ctx, t, apiClient, netName, net.WithMacvlan("dm-dummy0.60"))

	ctrID := container.Run(ctx, t, apiClient,
		container.WithCmd("ip", "-o", "link", "show", "foobar"),
		container.WithEndpointSettings(netName, &network.EndpointSettings{
			DriverOpts: map[string]string{
				netlabel.Ifname: "foobar",
			},
		}))
	defer container.Remove(ctx, t, apiClient, ctrID, client.ContainerRemoveOptions{Force: true})

	out, err := container.Output(ctx, apiClient, ctrID)
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(out.Stdout, ": foobar@if"), "expected ': foobar@if' in 'ip link show':\n%s", out.Stdout)
}

// TestParentDown checks that when a macvlan's parent is down, a container can still
// be attached.
// Regression test for https://github.com/moby/moby/issues/49593
func TestParentDown(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	const tap = "dummytap0"
	res := icmd.RunCommand("ip", "tuntap", "add", "mode", "tap", tap)
	res.Assert(t, icmd.Success)
	defer icmd.RunCommand("ip", "link", "del", tap)

	const netName = "testnet-macvlan"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithMacvlan(tap),
		net.WithIPv6(),
	)

	ctrID := container.Run(ctx, t, apiClient, container.WithNetworkMode(netName))
	defer container.Remove(ctx, t, apiClient, ctrID, client.ContainerRemoveOptions{Force: true})
}
