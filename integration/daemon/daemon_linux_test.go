package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"context"
	"net"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

func TestDaemonDefaultBridgeWithFixedCidrButNoBip(t *testing.T) {
	ctx := testutil.StartSpan(baseContext, t)

	bridgeName := "ext-bridge1"
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_CREATE_DEFAULT_BRIDGE="+bridgeName))
	defer func() {
		d.Stop(t)
		d.Cleanup(t)
	}()

	defer func() {
		// No need to clean up when running this test in rootless mode, as the
		// interface is deleted when the daemon is stopped and the netns
		// reclaimed by the kernel.
		if !testEnv.IsRootless() {
			deleteInterface(t, bridgeName)
		}
	}()
	d.StartWithBusybox(ctx, t, "--bridge", bridgeName, "--fixed-cidr", "192.168.130.0/24")
}

// Test fixed-cidr and bip options, with various addresses on the bridge
// before the daemon starts.
func TestDaemonDefaultBridgeIPAM_Docker0(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't create test bridge in rootless namespace")
	ctx := testutil.StartSpan(baseContext, t)

	testcases := []defaultBridgeIPAMTestCase{
		{
			name:       "fixed-cidr only",
			daemonArgs: []string{"--fixed-cidr", "192.168.176.0/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/24",
					IPRange: "192.168.176.0/24",
				},
			},
		},
		{
			name:       "bip only",
			daemonArgs: []string{"--bip", "192.168.176.88/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/24",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "existing bridge address only",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/24",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "fixed-cidr within old bridge subnet",
			initialBridgeAddrs: []string{"192.168.176.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/24"},
			// There's no --bip to dictate the subnet, so it's derived from an
			// existing bridge address. If fixed-cidr's subnet is made smaller
			// following a daemon restart, a user might reasonably expect the
			// default bridge network's subnet to shrink to match. However,
			// that has not been the behaviour - instead, only the allocatable
			// range is reduced (as would happen with a user-managed bridge).
			// In this case, if the user wants a smaller subnet, their options
			// are to delete docker0, or supply a --bip. A change in this subtle
			// behaviour might be best. But, it's probably not causing problems,
			// and it'd be a breaking change for anyone relying on the existing
			// behaviour.
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/20",
					IPRange: "192.168.176.0/24",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "fixed-cidr within old bridge subnet with new bip",
			initialBridgeAddrs: []string{"192.168.176.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/24", "--bip", "192.168.176.99/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/24",
					IPRange: "192.168.176.0/24",
					Gateway: "192.168.176.99",
				},
			},
		},
		{
			name:               "old bridge subnet within fixed-cidr",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/20"},
			expIPAMConfig: []network.IPAMConfig{
				{
					// FIXME(robmry) - subnet didn't change, allocatable range
					//   is bigger than the subnet.
					Subnet:  "192.168.176.0/24",
					IPRange: "192.168.176.0/20",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "old bridge subnet outside fixed-cidr",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.177.0/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					// FIXME(robmry) - subnet and bridge address haven't changed,
					//   and the allocatable range is outside the subnet.
					Subnet:  "192.168.176.0/24",
					IPRange: "192.168.177.0/24",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "old bridge subnet outside fixed-cidr with bip",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.177.0/24", "--bip", "192.168.177.99/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.177.0/24",
					IPRange: "192.168.177.0/24",
					Gateway: "192.168.177.99",
				},
			},
		},
	}
	for _, tc := range testcases {
		testDefaultBridgeIPAM(ctx, t, tc)
	}
}

// Like TestDaemonUserDefaultBridgeIPAMDocker0, but with a user-defined/supplied
// bridge, instead of docker0.
func TestDaemonDefaultBridgeIPAM_UserBr(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't create test bridge in rootless namespace")
	ctx := testutil.StartSpan(baseContext, t)

	testcases := []defaultBridgeIPAMTestCase{
		{
			name:               "bridge only",
			initialBridgeAddrs: []string{"192.168.176.88/20"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/20",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:       "fixed-cidr only",
			daemonArgs: []string{"--fixed-cidr", "192.168.176.0/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/24",
					IPRange: "192.168.176.0/24",
				},
			},
		},
		{
			name:               "fcidr in bridge subnet and bridge ip in fcidr",
			initialBridgeAddrs: []string{"192.168.160.88/20", "192.168.176.88/20", "192.168.192.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/24"},
			// Selected bip should be the one within fixed-cidr
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet:  "192.168.176.0/20",
					IPRange: "192.168.176.0/24",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "fcidr in bridge subnet and bridge ip not in fcidr",
			initialBridgeAddrs: []string{"192.168.160.88/20", "192.168.176.88/20", "192.168.192.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.177.0/24"},
			expIPAMConfig: []network.IPAMConfig{
				{
					// FIXME(robmry) - selected subnet should be the one that encompasses
					//  fixed-cidr, allocatable range is outside the subnet.
					Subnet:  "192.168.160.0/20",
					IPRange: "192.168.177.0/24",
					Gateway: "192.168.160.88",
				},
			},
		},
		{
			name:               "fixed-cidr bigger than bridge subnet",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/20"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet: "192.168.176.0/24",
					// FIXME(robmry) - allocatable range is bigger than the subnet.
					IPRange: "192.168.176.0/20",
					Gateway: "192.168.176.88",
				},
			},
		},
		{
			name:               "no bridge ip within fixed-cidr",
			initialBridgeAddrs: []string{"192.168.160.88/20", "192.168.192.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/24"},
			// Selected bip should be the one within fixed-cidr
			expIPAMConfig: []network.IPAMConfig{
				{
					// FIXME(robmry) - allocatable range outside subnet.
					Subnet:  "192.168.160.0/20",
					IPRange: "192.168.176.0/24",
					Gateway: "192.168.160.88",
				},
			},
		},
		{
			name:               "fixed-cidr contains bridge subnet",
			initialBridgeAddrs: []string{"192.168.177.1/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/20"},
			expIPAMConfig: []network.IPAMConfig{
				{
					Subnet: "192.168.177.0/24",
					// FIXME(robmry) - allocatable range is bigger than subnet
					IPRange: "192.168.176.0/20",
					Gateway: "192.168.177.1",
				},
			},
		},
	}
	for _, tc := range testcases {
		tc.userDefinedBridge = true
		testDefaultBridgeIPAM(ctx, t, tc)
	}
}

type defaultBridgeIPAMTestCase struct {
	name               string
	userDefinedBridge  bool
	initialBridgeAddrs []string
	daemonArgs         []string
	expIPAMConfig      []network.IPAMConfig
}

func testDefaultBridgeIPAM(ctx context.Context, t *testing.T, tc defaultBridgeIPAMTestCase) {
	t.Run(tc.name, func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)
		const bridgeName = "br-dbi"

		createBridge(t, bridgeName, tc.initialBridgeAddrs)
		defer deleteInterface(t, bridgeName)

		var dOpts []daemon.Option
		dArgs := tc.daemonArgs
		if tc.userDefinedBridge {
			// If a bridge is supplied by the user, the daemon should use its addresses
			// to infer --bip (which cannot be specified).
			dArgs = append(dArgs, []string{"--bridge", bridgeName}...)
		} else {
			// The bridge is created and managed by docker, it's always called "docker0",
			// unless this test-only env var is set - to avoid conflict with the docker0
			// belonging to the daemon started in CI runs.
			dOpts = append(dOpts, daemon.WithEnvVars("DOCKER_TEST_CREATE_DEFAULT_BRIDGE="+bridgeName))
		}

		d := daemon.New(t, dOpts...)
		defer func() {
			d.Stop(t)
			d.Cleanup(t)
		}()
		d.StartWithBusybox(ctx, t, dArgs...)
		c := d.NewClientT(t)
		defer c.Close()

		insp, err := c.NetworkInspect(ctx, network.NetworkBridge, network.InspectOptions{})
		assert.NilError(t, err)
		assert.Check(t, is.DeepEqual(insp.IPAM.Config, tc.expIPAMConfig))
	})
}

func createBridge(t *testing.T, ifName string, addrs []string) {
	t.Helper()

	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifName,
		},
	}

	err := netlink.LinkAdd(link)
	assert.NilError(t, err)
	for _, addr := range addrs {
		ip, ipNet, err := net.ParseCIDR(addr)
		assert.NilError(t, err)
		ipNet.IP = ip
		err = netlink.AddrAdd(link, &netlink.Addr{IPNet: ipNet})
		assert.NilError(t, err)
	}
}

func deleteInterface(t *testing.T, ifName string) {
	icmd.RunCommand("ip", "link", "delete", ifName).Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "-t", "nat", "--flush").Assert(t, icmd.Success)
	icmd.RunCommand("iptables", "--flush").Assert(t, icmd.Success)
}
