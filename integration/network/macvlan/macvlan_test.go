package macvlan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	n "github.com/docker/docker/integration/network"
	"github.com/docker/docker/internal/test/daemon"
	"github.com/gotestyourself/gotestyourself/assert"
	"github.com/gotestyourself/gotestyourself/skip"
)

func TestDockerNetworkMacvlanPersistance(t *testing.T) {
	// verify the driver automatically provisions the 802.1q link (dm-dummy0.60)
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())
	skip.If(t, !macvlanKernelSupport(), "Kernel doesn't support macvlan")

	d := daemon.New(t)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	master := "dm-dummy0"
	n.CreateMasterDummy(t, master)
	defer n.DeleteInterface(t, master)

	client, err := d.NewClient()
	assert.NilError(t, err)

	_, err = client.NetworkCreate(context.Background(), "dm-persist", types.NetworkCreate{
		Driver: "macvlan",
		Options: map[string]string{
			"parent": "dm-dummy0.60",
		},
	})
	assert.NilError(t, err)
	assert.Check(t, n.IsNetworkAvailable(client, "dm-persist"))
	d.Restart(t)
	assert.Check(t, n.IsNetworkAvailable(client, "dm-persist"))
}

func TestDockerNetworkMacvlan(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsRemoteDaemon())
	skip.If(t, !macvlanKernelSupport(), "Kernel doesn't support macvlan")

	for _, tc := range []struct {
		name string
		test func(client.APIClient) func(*testing.T)
	}{
		{
			name: "Subinterface",
			test: testMacvlanSubinterface,
		}, {
			name: "OverlapParent",
			test: testMacvlanOverlapParent,
		}, {
			name: "NilParent",
			test: testMacvlanNilParent,
		}, {
			name: "InternalMode",
			test: testMacvlanInternalMode,
		}, {
			name: "Addressing",
			test: testMacvlanAddressing,
		},
	} {
		d := daemon.New(t)
		d.StartWithBusybox(t)

		client, err := d.NewClient()
		assert.NilError(t, err)

		t.Run(tc.name, tc.test(client))

		d.Stop(t)
		// FIXME(vdemeester) clean network
	}
}

func testMacvlanOverlapParent(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		// verify the same parent interface cannot be used if already in use by an existing network
		master := "dm-dummy0"
		n.CreateMasterDummy(t, master)
		defer n.DeleteInterface(t, master)

		_, err := client.NetworkCreate(context.Background(), "dm-subinterface", types.NetworkCreate{
			Driver: "macvlan",
			Options: map[string]string{
				"parent": "dm-dummy0.40",
			},
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dm-subinterface"))

		_, err = client.NetworkCreate(context.Background(), "dm-parent-net-overlap", types.NetworkCreate{
			Driver: "macvlan",
			Options: map[string]string{
				"parent": "dm-dummy0.40",
			},
		})
		assert.Check(t, err != nil)
		// delete the network while preserving the parent link
		err = client.NetworkRemove(context.Background(), "dm-subinterface")
		assert.NilError(t, err)

		assert.Check(t, n.IsNetworkNotAvailable(client, "dm-subinterface"))
		// verify the network delete did not delete the predefined link
		n.LinkExists(t, "dm-dummy0")
	}
}

func testMacvlanSubinterface(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		// verify the same parent interface cannot be used if already in use by an existing network
		master := "dm-dummy0"
		n.CreateMasterDummy(t, master)
		defer n.DeleteInterface(t, master)
		n.CreateVlanInterface(t, master, "dm-dummy0.20", "20")

		_, err := client.NetworkCreate(context.Background(), "dm-subinterface", types.NetworkCreate{
			Driver: "macvlan",
			Options: map[string]string{
				"parent": "dm-dummy0.20",
			},
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dm-subinterface"))

		// delete the network while preserving the parent link
		err = client.NetworkRemove(context.Background(), "dm-subinterface")
		assert.NilError(t, err)

		assert.Check(t, n.IsNetworkNotAvailable(client, "dm-subinterface"))
		// verify the network delete did not delete the predefined link
		n.LinkExists(t, "dm-dummy0.20")
	}
}

func testMacvlanNilParent(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		// macvlan bridge mode - dummy parent interface is provisioned dynamically
		_, err := client.NetworkCreate(context.Background(), "dm-nil-parent", types.NetworkCreate{
			Driver: "macvlan",
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dm-nil-parent"))

		ctx := context.Background()
		id1 := container.Run(t, ctx, client, container.WithNetworkMode("dm-nil-parent"))
		id2 := container.Run(t, ctx, client, container.WithNetworkMode("dm-nil-parent"))

		_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
		assert.Check(t, err == nil)
	}
}

func testMacvlanInternalMode(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		// macvlan bridge mode - dummy parent interface is provisioned dynamically
		_, err := client.NetworkCreate(context.Background(), "dm-internal", types.NetworkCreate{
			Driver:   "macvlan",
			Internal: true,
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dm-internal"))

		ctx := context.Background()
		id1 := container.Run(t, ctx, client, container.WithNetworkMode("dm-internal"))
		id2 := container.Run(t, ctx, client, container.WithNetworkMode("dm-internal"))

		timeoutCtx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()
		_, err = container.Exec(timeoutCtx, client, id1, []string{"ping", "-c", "1", "-w", "1", "8.8.8.8"})
		// FIXME(vdemeester) check the time of error ?
		assert.Check(t, err != nil)
		assert.Check(t, timeoutCtx.Err() == context.DeadlineExceeded)

		_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", id1})
		assert.Check(t, err == nil)
	}
}

func testMacvlanMultiSubnet(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		_, err := client.NetworkCreate(context.Background(), "dualstackbridge", types.NetworkCreate{
			Driver:     "macvlan",
			EnableIPv6: true,
			IPAM: &network.IPAM{
				Config: []network.IPAMConfig{
					{
						Subnet:     "172.28.100.0/24",
						AuxAddress: map[string]string{},
					},
					{
						Subnet:     "172.28.102.0/24",
						Gateway:    "172.28.102.254",
						AuxAddress: map[string]string{},
					},
					{
						Subnet:     "2001:db8:abc2::/64",
						AuxAddress: map[string]string{},
					},
					{
						Subnet:     "2001:db8:abc4::/64",
						Gateway:    "2001:db8:abc4::254",
						AuxAddress: map[string]string{},
					},
				},
			},
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dualstackbridge"))

		// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.100.0/24 and 2001:db8:abc2::/64
		ctx := context.Background()
		id1 := container.Run(t, ctx, client,
			container.WithNetworkMode("dualstackbridge"),
			container.WithIPv4("dualstackbridge", "172.28.100.20"),
			container.WithIPv6("dualstackbridge", "2001:db8:abc2::20"),
		)
		id2 := container.Run(t, ctx, client,
			container.WithNetworkMode("dualstackbridge"),
			container.WithIPv4("dualstackbridge", "172.28.100.21"),
			container.WithIPv6("dualstackbridge", "2001:db8:abc2::21"),
		)
		c1, err := client.ContainerInspect(ctx, id1)
		assert.NilError(t, err)

		// verify ipv4 connectivity to the explicit --ipv address second to first
		_, err = container.Exec(ctx, client, id2, []string{"ping", "-c", "1", c1.NetworkSettings.Networks["dualstackbridge"].IPAddress})
		assert.NilError(t, err)
		// verify ipv6 connectivity to the explicit --ipv6 address second to first
		_, err = container.Exec(ctx, client, id2, []string{"ping6", "-c", "1", c1.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address})
		assert.NilError(t, err)

		// start dual stack containers and verify the user specified --ip and --ip6 addresses on subnets 172.28.102.0/24 and 2001:db8:abc4::/64
		id3 := container.Run(t, ctx, client,
			container.WithNetworkMode("dualstackbridge"),
			container.WithIPv4("dualstackbridge", "172.28.102.20"),
			container.WithIPv6("dualstackbridge", "2001:db8:abc4::20"),
		)
		id4 := container.Run(t, ctx, client,
			container.WithNetworkMode("dualstackbridge"),
			container.WithIPv4("dualstackbridge", "172.28.102.21"),
			container.WithIPv6("dualstackbridge", "2001:db8:abc4::21"),
		)
		c3, err := client.ContainerInspect(ctx, id3)
		assert.NilError(t, err)

		// verify ipv4 connectivity to the explicit --ipv address from third to fourth
		_, err = container.Exec(ctx, client, id4, []string{"ping", "-c", "1", c3.NetworkSettings.Networks["dualstackbridge"].IPAddress})
		assert.NilError(t, err)
		// verify ipv6 connectivity to the explicit --ipv6 address from third to fourth
		_, err = container.Exec(ctx, client, id4, []string{"ping6", "-c", "1", c3.NetworkSettings.Networks["dualstackbridge"].GlobalIPv6Address})
		assert.NilError(t, err)

		// Inspect the v4 gateway to ensure the proper default GW was assigned
		assert.Equal(t, c1.NetworkSettings.Networks["dualstackbridge"].Gateway, "172.28.100.1")
		// Inspect the v6 gateway to ensure the proper default GW was assigned
		assert.Equal(t, c1.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, "2001:db8:abc2::1")
		// Inspect the v4 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Equal(t, c3.NetworkSettings.Networks["dualstackbridge"].Gateway, "172.28.102.254")
		// Inspect the v6 gateway to ensure the proper explicitly assigned default GW was assigned
		assert.Equal(t, c3.NetworkSettings.Networks["dualstackbridge"].IPv6Gateway, "2001:db8.abc4::254")
	}
}

func testMacvlanAddressing(client client.APIClient) func(*testing.T) {
	return func(t *testing.T) {
		// Ensure the default gateways, next-hops and default dev devices are properly set
		_, err := client.NetworkCreate(context.Background(), "dualstackbridge", types.NetworkCreate{
			Driver:     "macvlan",
			EnableIPv6: true,
			Options: map[string]string{
				"macvlan_mode": "bridge",
			},
			IPAM: &network.IPAM{
				Config: []network.IPAMConfig{
					{
						Subnet:     "172.28.130.0/24",
						AuxAddress: map[string]string{},
					},
					{
						Subnet:     "2001:db8:abca::/64",
						Gateway:    "2001:db8:abca::254",
						AuxAddress: map[string]string{},
					},
				},
			},
		})
		assert.NilError(t, err)
		assert.Check(t, n.IsNetworkAvailable(client, "dualstackbridge"))

		ctx := context.Background()
		id1 := container.Run(t, ctx, client,
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
}

// ensure Kernel version is >= v3.9 for macvlan support
func macvlanKernelSupport() bool {
	return n.CheckKernelMajorVersionGreaterOrEqualThen(3, 9)
}
