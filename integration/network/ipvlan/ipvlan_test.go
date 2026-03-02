//go:build !windows

package ipvlan

import (
	"context"
	"fmt"
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
		test func(*testing.T, context.Context, client.APIClient)
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
			name: "L2Gateway",
			test: testIpvlanL2Addressing,
		}, {
			name: "L3Addressing",
			test: testIpvlanL3Addressing,
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

func testIpvlanSubinterface(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	master := "di-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	netName := "di-subinterface"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("di-dummy0.60", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	// delete the network while preserving the parent link
	_, err := apiClient.NetworkRemove(ctx, netName, client.NetworkRemoveOptions{})
	assert.NilError(t, err)

	assert.Check(t, n.IsNetworkNotAvailable(ctx, apiClient, netName))
	// verify the network delete did not delete the predefined link
	n.LinkExists(ctx, t, "di-dummy0")
}

func testIpvlanOverlapParent(t *testing.T, ctx context.Context, client client.APIClient) {
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

func testIpvlanL2NilParent(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	// ipvlan l2 mode - dummy parent interface is provisioned dynamically
	netName := "di-nil-parent"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	id1 := container.Run(ctx, t, apiClient, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, apiClient, container.WithNetworkMode(netName))

	_, err := container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL2InternalMode(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	netName := "di-internal"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("", ""),
		net.WithInternal(),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	id1 := container.Run(ctx, t, apiClient, container.WithNetworkMode(netName))
	id2 := container.Run(ctx, t, apiClient, container.WithNetworkMode(netName))

	result, _ := container.Exec(ctx, apiClient, id1, []string{"ping", "-c", "1", "8.8.8.8"})
	assert.Check(t, is.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL3NilParent(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	netName := "di-nil-parent-l3"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("", "l3"),
		net.WithIPAM("172.28.230.0/24", ""),
		net.WithIPAM("172.28.220.0/24", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	id1 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.220.10"),
	)
	id2 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.230.10"),
	)

	_, err := container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL3InternalMode(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	netName := "di-internal-l3"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("", "l3"),
		net.WithInternal(),
		net.WithIPAM("172.28.230.0/24", ""),
		net.WithIPAM("172.28.220.0/24", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	id1 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.220.10"),
	)
	id2 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.230.10"),
	)

	result, _ := container.Exec(ctx, apiClient, id1, []string{"ping", "-c", "1", "8.8.8.8"})
	assert.Check(t, is.Contains(result.Combined(), "Network is unreachable"))

	_, err := container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", id1})
	assert.NilError(t, err)
}

func testIpvlanL2MultiSubnetWithParent(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)
	testIpvlanL2MultiSubnet(t, ctx, apiClient, parentIfName)
}

func testIpvlanL2MultiSubnetNoParent(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	testIpvlanL2MultiSubnet(t, ctx, apiClient, "")
}

func testIpvlanL2MultiSubnet(t *testing.T, ctx context.Context, apiClient client.APIClient, parent string) {
	netName := "dualstackl2"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan(parent, ""),
		net.WithIPv6(),
		net.WithIPAM("172.28.200.0/24", ""),
		net.WithIPAM("172.28.202.0/24", "172.28.202.254"),
		net.WithIPAM("2001:db8:abc8::/64", ""),
		net.WithIPAM("2001:db8:abc6::/64", "2001:db8:abc6::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.200.20"),
		container.WithIPv6(netName, "2001:db8:abc8::20"),
	)
	id2 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.200.21"),
		container.WithIPv6(netName, "2001:db8:abc8::21"),
	)
	c1, err := apiClient.ContainerInspect(ctx, id1, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	// Inspect the v4 gateway to ensure no default GW was assigned
	assert.Check(t, !c1.Container.NetworkSettings.Networks[netName].Gateway.IsValid())
	// Inspect the v6 gateway to ensure no default GW was assigned
	assert.Check(t, !c1.Container.NetworkSettings.Networks[netName].IPv6Gateway.IsValid())

	// verify ipv4 connectivity to the explicit --ip address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", c1.Container.NetworkSettings.Networks[netName].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping6", "-c", "1", c1.Container.NetworkSettings.Networks[netName].GlobalIPv6Address.String()})
	assert.NilError(t, err)

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.202.20"),
		container.WithIPv6(netName, "2001:db8:abc6::20"),
	)
	id4 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.202.21"),
		container.WithIPv6(netName, "2001:db8:abc6::21"),
	)
	c3, err := apiClient.ContainerInspect(ctx, id3, client.ContainerInspectOptions{})
	assert.NilError(t, err)
	if parent == "" {
		// Inspect the v4 gateway to ensure no default GW was assigned
		assert.Check(t, !c3.Container.NetworkSettings.Networks[netName].Gateway.IsValid())
		// Inspect the v6 gateway to ensure no default GW was assigned
		assert.Check(t, !c3.Container.NetworkSettings.Networks[netName].IPv6Gateway.IsValid())
	} else {
		// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.Container.NetworkSettings.Networks[netName].Gateway, netip.MustParseAddr("172.28.202.254")))
		// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Check(t, is.Equal(c3.Container.NetworkSettings.Networks[netName].IPv6Gateway, netip.MustParseAddr("2001:db8:abc6::254")))
	}

	// verify ipv4 connectivity to the explicit --ip address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping", "-c", "1", c3.Container.NetworkSettings.Networks[netName].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ip6 address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping6", "-c", "1", c3.Container.NetworkSettings.Networks[netName].GlobalIPv6Address.String()})
	assert.NilError(t, err)
}

func testIpvlanL3MultiSubnet(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	netName := "dualstackl3"
	net.CreateNoError(ctx, t, apiClient, netName,
		net.WithIPvlan("", "l3"),
		net.WithIPv6(),
		net.WithIPAM("172.28.10.0/24", ""),
		net.WithIPAM("172.28.12.0/24", "172.28.12.254"),
		net.WithIPAM("2001:db8:abc9::/64", ""),
		net.WithIPAM("2001:db8:abc7::/64", "2001:db8:abc7::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netName))

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
	id1 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.10.20"),
		container.WithIPv6(netName, "2001:db8:abc9::20"),
	)
	id2 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.10.21"),
		container.WithIPv6(netName, "2001:db8:abc9::21"),
	)
	c1, err := apiClient.ContainerInspect(ctx, id1, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	// verify ipv4 connectivity to the explicit --ipv address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping", "-c", "1", c1.Container.NetworkSettings.Networks[netName].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address second to first
	_, err = container.Exec(ctx, apiClient, id2, []string{"ping6", "-c", "1", c1.Container.NetworkSettings.Networks[netName].GlobalIPv6Address.String()})
	assert.NilError(t, err)

	// start dual stack containers and verify the user-specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
	id3 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.12.20"),
		container.WithIPv6(netName, "2001:db8:abc7::20"),
	)
	id4 := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netName),
		container.WithIPv4(netName, "172.28.12.21"),
		container.WithIPv6(netName, "2001:db8:abc7::21"),
	)
	c3, err := apiClient.ContainerInspect(ctx, id3, client.ContainerInspectOptions{})
	assert.NilError(t, err)

	// verify ipv4 connectivity to the explicit --ipv address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping", "-c", "1", c3.Container.NetworkSettings.Networks[netName].IPAddress.String()})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address from third to fourth
	_, err = container.Exec(ctx, apiClient, id4, []string{"ping6", "-c", "1", c3.Container.NetworkSettings.Networks[netName].GlobalIPv6Address.String()})
	assert.NilError(t, err)

	// Inspect the v4 gateway to ensure no next hop is assigned in L3 mode
	assert.Check(t, !c1.Container.NetworkSettings.Networks[netName].Gateway.IsValid())
	// Inspect the v6 gateway to ensure the explicitly specified default GW is ignored per L3 mode enabled
	assert.Check(t, !c1.Container.NetworkSettings.Networks[netName].IPv6Gateway.IsValid())
	// Inspect the v4 gateway to ensure no next hop is assigned in L3 mode
	assert.Check(t, !c3.Container.NetworkSettings.Networks[netName].Gateway.IsValid())
	// Inspect the v6 gateway to ensure the explicitly specified default GW is ignored per L3 mode enabled
	assert.Check(t, !c3.Container.NetworkSettings.Networks[netName].IPv6Gateway.IsValid())
}

// Verify ipvlan l2 mode sets the proper default gateway routes via netlink
// for either an explicitly set route by the user or inferred via default IPAM
func testIpvlanL2Addressing(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	netNameL2 := "dualstackl2"
	net.CreateNoError(ctx, t, apiClient, netNameL2,
		net.WithIPvlan(parentIfName, "l2"),
		net.WithIPv6(),
		net.WithIPAM("172.28.140.0/24", "172.28.140.254"),
		net.WithIPAM("2001:db8:abcb::/64", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netNameL2))

	id := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netNameL2),
	)
	// Check the supplied IPv4 gateway address is used in a default route.
	result, err := container.Exec(ctx, apiClient, id, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default via 172.28.140.254 dev eth0"))
	// No gateway address was supplied for IPv6, check that no default gateway was set up.
	result, err = container.Exec(ctx, apiClient, id, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, !strings.Contains(result.Combined(), "default via"),
		"result: %s", result.Combined())
}

// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
func testIpvlanL3Addressing(t *testing.T, ctx context.Context, apiClient client.APIClient) {
	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	netNameL3 := "dualstackl3"
	net.CreateNoError(ctx, t, apiClient, netNameL3,
		net.WithIPvlan(parentIfName, "l3"),
		net.WithIPv6(),
		net.WithIPAM("172.28.160.0/24", "172.28.160.254"),
		net.WithIPAM("2001:db8:abcd::/64", "2001:db8:abcd::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, apiClient, netNameL3))

	id := container.Run(ctx, t, apiClient,
		container.WithNetworkMode(netNameL3),
	)
	// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err := container.Exec(ctx, apiClient, id, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default dev eth0"))
	// Validate ipvlan l3 mode sets the v6 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err = container.Exec(ctx, apiClient, id, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, is.Contains(result.Combined(), "default dev eth0"))
}

// Check that an ipvlan interface with '--ipv6=false' doesn't get kernel-assigned
// IPv6 addresses, but the loopback interface does still have an IPv6 address ('::1').
// Also check that with '--ipv4=false', there's no IPAM-assigned IPv4 address.
func TestIpvlanIPAM(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := testutil.StartSpan(baseContext, t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	tests := []struct {
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
		subnetv4 = netip.MustParsePrefix("10.42.42.0/24")
		subnetv6 = netip.MustParsePrefix("2001:db8:abcd::/64")
	)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			c := d.NewClientT(t, client.WithAPIVersion(tc.apiVersion))

			netOpts := []func(*client.NetworkCreateOptions){
				net.WithIPvlan("", "l3"),
				net.WithIPv4(tc.enableIPv4),
				net.WithIPAMConfig(
					network.IPAMConfig{
						Subnet: subnetv4,
					},
					network.IPAMConfig{
						Subnet:  subnetv6,
						IPRange: netip.MustParsePrefix("2001:db8:abcd::100/120"),
					},
				),
			}
			if tc.enableIPv6 {
				netOpts = append(netOpts, net.WithIPv6())
			}

			const netName = "ipvlannet"
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
					IPsInUse:            3, // network, broadcast, container
					DynamicIPsAvailable: 253,
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
					DynamicIPsAvailable: 255,
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
			cc.Close()
			cc = d.NewClientT(t, client.WithAPIVersion("1.51"))
			res, err = cc.NetworkInspect(ctx, netName, client.NetworkInspectOptions{})
			assert.Check(t, err)
			assert.Check(t, res.Network.Status == nil)
			cc.Close()
		})
	}
}

// IPVLAN networks are allowed to be assigned IPAM subnets that overlap with
// other IPVLAN networks' IPAM subnets. But no two IPVLAN endpoints may be
// assigned the same address, even when the endpoints are attached to different
// networks. The assignment of an address to an endpoint on one network may
// therefore reduce the number of available addresses to assign to other
// networks' endpoints.
func TestIpvlanIPAMOverlap(t *testing.T) {
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
		netName1 = "ipvlannet1"
		netName2 = "ipvlannet2"
		netName3 = "ipvlannet3"
	)
	cidrv4 := netip.MustParsePrefix("192.168.0.0/24")
	cidrv6 := netip.MustParsePrefix("2001:db8:abcd::/64")

	net.CreateNoError(ctx, t, c, netName1,
		net.WithIPvlan("", "l3"),
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
		net.WithIPvlan("", "l3"),
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
		net.WithIPvlan("", "l3"),
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

	d := daemon.New(t, daemon.WithResolvConf(net.GenResolvConf("127.0.0.1")))
	d.StartWithBusybox(ctx, t)
	t.Cleanup(func() { d.Stop(t) })
	c := d.NewClientT(t)

	const parentIfName = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfName)
	defer n.DeleteInterface(ctx, t, parentIfName)

	const netName = "ipvlan-dns-net"

	tests := []struct {
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
		for _, tc := range tests {
			name := fmt.Sprintf("Mode=%v/HasParent=%v/Internal=%v", mode, tc.parent != "", tc.internal)
			t.Run(name, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				createOpts := []func(*client.NetworkCreateOptions){
					net.WithIPvlan(tc.parent, mode),
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
}

// TestPointToPoint checks that no gateway is reserved for an ipvlan network unless needed.
func TestPointToPoint(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't see dummy parent interface from rootless netns")

	ctx := testutil.StartSpan(baseContext, t)
	apiClient := testEnv.APIClient()

	const parentIfname = "di-dummy0"
	n.CreateMasterDummy(ctx, t, parentIfname)
	defer n.DeleteInterface(ctx, t, parentIfname)

	tests := []struct {
		name   string
		parent string
		mode   string
	}{
		{
			// An ipvlan network with no parent interface is "internal".
			name: "internal",
			mode: "l2",
		},
		{
			// An L3 ipvlan does not need a gateway, because it's L3 (so, can't
			// resolve a next-hop address). A "/31" ipvlan with two containers
			// may not be particularly useful, but the check is that no address is
			// reserved for a gateway in an L3 network.
			name:   "l3",
			parent: parentIfname,
			mode:   "l3",
		},
		{
			name:   "l3s",
			parent: parentIfname,
			mode:   "l3s",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			const netName = "p2pipvlan"
			net.CreateNoError(ctx, t, apiClient, netName,
				net.WithIPvlan(tc.parent, tc.mode),
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
		})
	}
}

func TestEndpointWithCustomIfname(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, testEnv.IsRootless, "rootless mode has different view of network")

	ctx := setupTest(t)
	apiClient := testEnv.APIClient()

	// master dummy interface 'di' notation represent 'docker ipvlan'
	const master = "di-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)

	// create a network specifying the desired sub-interface name
	const netName = "ipvlan-custom-ifname"
	net.CreateNoError(ctx, t, apiClient, netName, net.WithIPvlan("di-dummy0.70", ""))

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
