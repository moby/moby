//go:build !windows

package ipvlan // import "github.com/docker/docker/integration/network/ipvlan"

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	dclient "github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	net "github.com/docker/docker/integration/internal/network"
	n "github.com/docker/docker/integration/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/skip"
)

func TestDockerNetworkIpvlanPersistance(t *testing.T) {
	// verify the driver automatically provisions the 802.1q link (di-dummy0.70)
	skip.If(t, testEnv.IsRemoteDaemon)
	skip.If(t, !ipvlanKernelSupport(t), "Kernel doesn't support ipvlan")

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
	skip.If(t, !ipvlanKernelSupport(t), "Kernel doesn't support ipvlan")

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
			name: "L2MultiSubnet",
			test: testIpvlanL2MultiSubnet,
		}, {
			name: "L3MultiSubnet",
			test: testIpvlanL3MultiSubnet,
		}, {
			name: "Addressing",
			test: testIpvlanAddressing,
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

func testIpvlanL2MultiSubnet(t *testing.T, ctx context.Context, client dclient.APIClient) {
	netName := "dualstackl2"
	net.CreateNoError(ctx, t, client, netName,
		net.WithIPvlan("", ""),
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

	// verify ipv4 connectivity to the explicit --ipv address second to first
	_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", c1.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address second to first
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

	// verify ipv4 connectivity to the explicit --ipv address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping", "-c", "1", c3.NetworkSettings.Networks[netName].IPAddress})
	assert.NilError(t, err)
	// verify ipv6 connectivity to the explicit --ipv6 address from third to fourth
	_, err = container.Exec(ctx, client, id4, []string{"ping6", "-c", "1", c3.NetworkSettings.Networks[netName].GlobalIPv6Address})
	assert.NilError(t, err)

	// Inspect the v4 gateway to ensure the proper default GW was assigned
	assert.Equal(t, c1.NetworkSettings.Networks[netName].Gateway, "172.28.200.1")
	// Inspect the v6 gateway to ensure the proper default GW was assigned
	assert.Equal(t, c1.NetworkSettings.Networks[netName].IPv6Gateway, "2001:db8:abc8::1")
	// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
	assert.Equal(t, c3.NetworkSettings.Networks[netName].Gateway, "172.28.202.254")
	// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
	assert.Equal(t, c3.NetworkSettings.Networks[netName].IPv6Gateway, "2001:db8:abc6::254")
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

func testIpvlanAddressing(t *testing.T, ctx context.Context, client dclient.APIClient) {
	// Verify ipvlan l2 mode sets the proper default gateway routes via netlink
	// for either an explicitly set route by the user or inferred via default IPAM
	netNameL2 := "dualstackl2"
	net.CreateNoError(ctx, t, client, netNameL2,
		net.WithIPvlan("", "l2"),
		net.WithIPv6(),
		net.WithIPAM("172.28.140.0/24", "172.28.140.254"),
		net.WithIPAM("2001:db8:abcb::/64", ""),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netNameL2))

	id1 := container.Run(ctx, t, client,
		container.WithNetworkMode(netNameL2),
	)
	// Validate ipvlan l2 mode defaults gateway sets the default IPAM next-hop inferred from the subnet
	result, err := container.Exec(ctx, client, id1, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default via 172.28.140.254 dev eth0"))
	// Validate ipvlan l2 mode sets the v6 gateway to the user specified default gateway/next-hop
	result, err = container.Exec(ctx, client, id1, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default via 2001:db8:abcb::1 dev eth0"))

	// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
	netNameL3 := "dualstackl3"
	net.CreateNoError(ctx, t, client, netNameL3,
		net.WithIPvlan("", "l3"),
		net.WithIPv6(),
		net.WithIPAM("172.28.160.0/24", "172.28.160.254"),
		net.WithIPAM("2001:db8:abcd::/64", "2001:db8:abcd::254"),
	)
	assert.Check(t, n.IsNetworkAvailable(ctx, client, netNameL3))

	id2 := container.Run(ctx, t, client,
		container.WithNetworkMode(netNameL3),
	)
	// Validate ipvlan l3 mode sets the v4 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err = container.Exec(ctx, client, id2, []string{"ip", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default dev eth0"))
	// Validate ipvlan l3 mode sets the v6 gateway to dev eth0 and disregards any explicit or inferred next-hops
	result, err = container.Exec(ctx, client, id2, []string{"ip", "-6", "route"})
	assert.NilError(t, err)
	assert.Check(t, strings.Contains(result.Combined(), "default dev eth0"))
}

var (
	once            sync.Once
	ipvlanSupported bool
)

// figure out if ipvlan is supported by the kernel
func ipvlanKernelSupport(t *testing.T) bool {
	once.Do(func() {
		// this may have the side effect of enabling the ipvlan module
		exec.Command("modprobe", "ipvlan").Run()
		_, err := os.Stat("/sys/module/ipvlan")
		if err == nil {
			ipvlanSupported = true
		} else if !os.IsNotExist(err) {
			t.Logf("WARNING: ipvlanKernelSupport: stat failed: %v\n", err)
		}
	})

	return ipvlanSupported
}
