package daemon // import "github.com/docker/docker/integration/daemon"

import (
	"context"
	"net"
	"net/netip"
	"slices"
	"testing"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/internal/nlwrap"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

// Check that the daemon will start with fixed-cidr set, but no bip.
// Regression test for https://github.com/moby/moby/issues/45356
func TestDaemonDefaultBridgeWithFixedCidrButNoBip(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't create L3 segment in rootless namespace")
	ctx := testutil.StartSpan(baseContext, t)

	host, cleanup := newHostInL3Seg(t, "fcnobip", "192.168.130.1", "fd69:d2cd:f7df::1")
	defer cleanup()

	host.Do(t, func() {
		// Run without OTel because there's no routing from this netns for it - which
		// means the daemon doesn't shut down cleanly, causing the test to fail.
		d := daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
		d.StartWithBusybox(ctx, t, "--fixed-cidr", "192.168.130.0/24")
		defer func() {
			d.Stop(t)
			d.Cleanup(t)
		}()
	})
}

// Test fixed-cidr and bip options, with various addresses on the bridge
// before the daemon starts.
func TestDaemonDefaultBridgeIPAM_Docker0(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't create L3 segment in rootless namespace")
	ctx := testutil.StartSpan(baseContext, t)

	testcases := []defaultBridgeIPAMTestCase{
		{
			name: "no config",
			// No config for the bridge, but override default-address-pools to
			// get a predictable result for IPv6 (rather than the daemon's ULA).
			daemonArgs: []string{
				"--default-address-pool", `base=192.168.176.0/20,size=24`,
				"--default-address-pool", `base=fdd1:8161:2d2c::/56,size=64`,
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", Gateway: "192.168.176.1"},
				{Subnet: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::1/64"},
			},
		},
		{
			name: "fixed-cidr only",
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", IPRange: "192.168.176.0/24"},
				{Subnet: "fdd1:8161:2d2c::/64", IPRange: "fdd1:8161:2d2c::/64"},
			},
		},
		{
			name: "bip only",
			daemonArgs: []string{
				"--bip", "192.168.176.88/24",
				"--bip6", "fdd1:8161:2d2c::8888/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::8888"},
			},
		},
		{
			name:               "existing bridge address only",
			initialBridgeAddrs: []string{"192.168.176.88/24", "fdd1:8161:2d2c::8888/64"},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::8888"},
			},
		},
		{
			name:               "fixed-cidr within old bridge subnet",
			initialBridgeAddrs: []string{"192.168.176.88/20", "fdd1:8161:2d2c::8888/56"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c::/64",
			},
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
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c::/56", IPRange: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::8888"},
			},
		},
		{
			name: "link-local fixed-cidr-v6",
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fe80::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", IPRange: "192.168.176.0/24"},
				{Subnet: "fe80::/64", IPRange: "fe80::/64", Gateway: llGwPlaceholder},
			},
		},
		{
			name:               "nonstandard link-local fixed-cidr-v6",
			initialBridgeAddrs: []string{"192.168.176.88/20", "fe80:1234::88/56"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fe80:1234::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fe80:1234::/56", IPRange: "fe80:1234::/64", Gateway: "fe80:1234::88"},
			},
		},
		{
			name:               "fixed-cidr within old bridge subnet with new bip",
			initialBridgeAddrs: []string{"192.168.176.88/20", "fdd1:8161:2d2c::/56"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24", "--bip", "192.168.176.99/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c::/64", "--bip6", "fdd1:8161:2d2c::9999/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", IPRange: "192.168.176.0/24", Gateway: "192.168.176.99"},
				{Subnet: "fdd1:8161:2d2c::/64", IPRange: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::9999"},
			},
		},
		{
			name:               "old bridge subnet within fixed-cidr",
			initialBridgeAddrs: []string{"192.168.176.88/24", "fdd1:8161:2d2c::8888/64"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/20",
				"--fixed-cidr-v6", "fdd1:8161:2d2c::/56",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/20", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c::/56", IPRange: "fdd1:8161:2d2c::/56", Gateway: "fdd1:8161:2d2c::8888"},
			},
		},
		{
			name:               "old bridge subnet outside fixed-cidr",
			initialBridgeAddrs: []string{"192.168.176.88/24", "fdd1:8161:2d2c::8888/64"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.177.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c:1::/64",
			},
			// The bridge's address/subnet should be ignored, this is a change
			// of fixed-cidr.
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.177.0/24", IPRange: "192.168.177.0/24"},
				{Subnet: "fdd1:8161:2d2c:1::/64", IPRange: "fdd1:8161:2d2c:1::/64"},
				// No Gateway is configured, because the address could not be learnt from the
				// bridge. An address will have been allocated but, because there's config (the
				// fixed-cidr), inspect shows just the config. (Surprisingly, when there's no
				// config at all, the inspect output still says its showing config but actually
				// shows the running state.) When the daemon is restarted, after a gateway
				// address has been assigned to the bridge, that address will become config - so
				// a Gateway address will show up in the inspect output.
			},
		},
		{
			name:               "old bridge subnet outside fixed-cidr with bip",
			initialBridgeAddrs: []string{"192.168.176.88/24", "fdd1:8161:2d2c::8888/64"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.177.0/24", "--bip", "192.168.177.99/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c:1::/64", "--bip6", "fdd1:8161:2d2c:1::9999/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.177.0/24", IPRange: "192.168.177.0/24", Gateway: "192.168.177.99"},
				{Subnet: "fdd1:8161:2d2c:1::/64", IPRange: "fdd1:8161:2d2c:1::/64", Gateway: "fdd1:8161:2d2c:1::9999"},
			},
		},
	}
	for _, tc := range testcases {
		tc.bridgeName = "docker0"
		testDefaultBridgeIPAM(ctx, t, tc)
	}
}

// Like TestDaemonUserDefaultBridgeIPAMDocker0, but with a user-defined/supplied
// bridge, instead of docker0.
func TestDaemonDefaultBridgeIPAM_UserBr(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "can't create L3 segment in rootless namespace")
	ctx := testutil.StartSpan(baseContext, t)

	testcases := []defaultBridgeIPAMTestCase{
		{
			name:               "bridge only",
			initialBridgeAddrs: []string{"192.168.176.88/20", "fdd1:8161:2d2c::8888/64"},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c::/64", Gateway: "fdd1:8161:2d2c::8888"},
			},
		},
		{
			name: "fixed-cidr only",
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/24", IPRange: "192.168.176.0/24"},
				{Subnet: "fdd1:8161:2d2c::/64", IPRange: "fdd1:8161:2d2c::/64"},
			},
		},
		{
			name: "fcidr in bridge subnet and bridge ip in fcidr",
			initialBridgeAddrs: []string{
				"192.168.160.88/20", "192.168.176.88/20", "192.168.192.88/20",
				"fdd1:8161:2d2c::8888/60", "fdd1:8161:2d2c:10::8888/60", "fdd1:8161:2d2c:20::8888/60",
			},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c:10::/64",
			},
			// Selected bip should be the one within fixed-cidr
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c:10::/60", IPRange: "fdd1:8161:2d2c:10::/64", Gateway: "fdd1:8161:2d2c:10::8888"},
			},
		},
		{
			name: "fcidr in bridge subnet and bridge ip not in fcidr",
			initialBridgeAddrs: []string{
				"192.168.160.88/20", "192.168.176.88/20", "192.168.192.88/20",
				"fdd1:8161:2d2c::8888/60", "fdd1:8161:2d2c:10::8888/60", "fdd1:8161:2d2c:20::8888/60",
			},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.177.0/24",
				"--fixed-cidr-v6", "fdd1:8161:2d2c:11::8888/64",
			},
			// Selected bridge subnet should be the one that encompasses fixed-cidr.
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.177.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fdd1:8161:2d2c:10::/60", IPRange: "fdd1:8161:2d2c:11::/64", Gateway: "fdd1:8161:2d2c:10::8888"},
			},
		},
		{
			name:               "link-local fixed-cidr-v6",
			initialBridgeAddrs: []string{"192.168.176.88/20"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fe80::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fe80::/64", IPRange: "fe80::/64", Gateway: llGwPlaceholder},
			},
		},
		{
			name:               "nonstandard link-local fixed-cidr-v6",
			initialBridgeAddrs: []string{"192.168.176.88/20", "fe80:1234::88/56"},
			daemonArgs: []string{
				"--fixed-cidr", "192.168.176.0/24",
				"--fixed-cidr-v6", "fe80:1234::/64",
			},
			expIPAMConfig: []network.IPAMConfig{
				{Subnet: "192.168.176.0/20", IPRange: "192.168.176.0/24", Gateway: "192.168.176.88"},
				{Subnet: "fe80:1234::/56", IPRange: "fe80:1234::/64", Gateway: "fe80:1234::88"},
			},
		},
		{
			name:               "fixed-cidr bigger than bridge subnet",
			initialBridgeAddrs: []string{"192.168.176.88/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/20"},
			ipv4Only:           true,
			// fixed-cidr (the range of allocatable addresses) is bigger than the
			// bridge subnet - this is a configuration error, but has historically
			// been allowed. Because IPRange is treated as an offset into Subnet, it
			// would normally result in a docker network that allocated addresses
			// within the selected subnet. So, fixed-cidr is dropped, making the
			// whole subnet allocatable.
			expIPAMConfig: []network.IPAMConfig{{Subnet: "192.168.176.0/24", Gateway: "192.168.176.88"}},
		},
		{
			name:               "no bridge ip within fixed-cidr",
			initialBridgeAddrs: []string{"192.168.160.88/20"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/24"},
			ipv4Only:           true,
			// fixed-cidr (the range of allocatable addresses) is outside the bridge
			// subnet - this is a configuration error, but has historically been
			// allowed. Because IPRange is treated as an offset into Subnet, it
			// would normally result in a docker network that allocated addresses
			// within the selected subnet. So, fixed-cidr is dropped, making the
			// whole subnet allocatable.
			expIPAMConfig: []network.IPAMConfig{{Subnet: "192.168.160.0/20", Gateway: "192.168.160.88"}},
		},
		{
			name:               "fixed-cidr contains bridge subnet",
			initialBridgeAddrs: []string{"192.168.177.1/24"},
			daemonArgs:         []string{"--fixed-cidr", "192.168.176.0/20"},
			// fixed-cidr (the range of allocatable addresses) is bigger than the
			// bridge subnet, and the bridge's address is not within fixed-cidr.
			// This is a configuration error, but has historically been allowed.
			// Because IPRange is treated as an offset into Subnet, it would
			// normally result in a docker network that allocated addresses
			// within the selected subnet. So, fixed-cidr is dropped, making the
			// whole subnet allocatable.
			ipv4Only:      true,
			expIPAMConfig: []network.IPAMConfig{{Subnet: "192.168.177.0/24", Gateway: "192.168.177.1"}},
		},

		{
			name:               "fixed-cidr-v6 bigger than bridge subnet",
			initialBridgeAddrs: []string{"fdd1:8161:2d2c::8888/64"},
			daemonArgs:         []string{"--fixed-cidr-v6", "fdd1:8161:2d2c::/60"},
			// fixed-cidr-v6 (the range of allocatable addresses) is bigger than the bridge
			// subnet - this is a configuration error. Unlike IPv4, it has not historically
			// been allowed, so it will prevent daemon startup.
			expStartErr: true,
		},
		{
			name:               "no bridge ip within fixed-cidr-v6",
			initialBridgeAddrs: []string{"fdd1:8161:2d2c::8888/60"},
			daemonArgs:         []string{"--fixed-cidr-v6", "fdd1:8161:2d2c:10::/64"},
			// fixed-cidr-v6 (the range of allocatable addresses) is outside the bridge subnet -
			// this is a configuration error. Unlike IPv4, it has not historically been
			// allowed, so it will prevent daemon startup.
			expStartErr: true,
		},
		{
			name:               "fixed-cidr-v6 contains bridge subnet",
			initialBridgeAddrs: []string{"fdd1:8161:2d2c:10::1/64"},
			daemonArgs:         []string{"--fixed-cidr-v6", "fdd1:8161:2d2c:10::/60"},
			// fixed-cidr-v6 (the range of allocatable addresses) is bigger than the
			// bridge subnet, and the bridge's address is not within fixed-cidr.
			// This is a configuration error, Unlike IPv4, it has not historically been
			// allowed, so it will prevent daemon startup.
			expStartErr: true,
		},
	}
	for _, tc := range testcases {
		tc.bridgeName = "br-dbi"
		tc.userDefinedBridge = true
		testDefaultBridgeIPAM(ctx, t, tc)
	}
}

// llGwPlaceholder can be used as a value for "Gateway" in expected IPAM config,
// before comparison with actual results it'll be replaced by the kernel assigned
// link local IPv6 address for the bridge.
const llGwPlaceholder = "ll-gateway-placeholder"

type defaultBridgeIPAMTestCase struct {
	name               string
	bridgeName         string
	userDefinedBridge  bool
	initialBridgeAddrs []string
	daemonArgs         []string
	ipv4Only           bool
	expStartErr        bool
	expIPAMConfig      []network.IPAMConfig
}

func testDefaultBridgeIPAM(ctx context.Context, t *testing.T, tc defaultBridgeIPAMTestCase) {
	t.Run(tc.name, func(t *testing.T) {
		ctx := testutil.StartSpan(ctx, t)

		// Run this test in its own network namespace because it messes with the default
		// bridge and, for example, iptables rules for the default bridge aren't deleted
		// because the network can't be deleted. Then, rules with old addresses may break
		// unrelated tests.
		host, cleanup := newHostInL3Seg(t, "defbripam", "192.168.131.1", "fdf9:2eb5:ba8c::1")
		defer cleanup()

		host.Do(t, func() {
			llAddr := createBridge(t, tc.bridgeName, tc.initialBridgeAddrs)

			var dArgs []string
			if !tc.ipv4Only {
				dArgs = append(tc.daemonArgs, "--ipv6")
			}
			if tc.userDefinedBridge {
				// If a bridge is supplied by the user, the daemon should use its addresses
				// to infer --bip (which cannot be specified).
				dArgs = append(dArgs, "--bridge", tc.bridgeName)
			}

			// Run without OTel because there's no routing from this netns for it - which
			// means the daemon doesn't shut down cleanly, causing the test to fail.
			d := daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
			defer func() {
				d.Stop(t)
				d.Cleanup(t)
			}()

			if tc.expStartErr {
				err := d.StartWithError(dArgs...)
				assert.Check(t, is.ErrorContains(err, "daemon exited during startup"))
				return
			}

			d.Start(t, dArgs...)
			c := d.NewClientT(t)
			defer c.Close()

			insp, err := c.NetworkInspect(ctx, network.NetworkBridge, network.InspectOptions{})
			assert.NilError(t, err)
			expIPAMConfig := slices.Clone(tc.expIPAMConfig)
			for i := range expIPAMConfig {
				if expIPAMConfig[i].Gateway == llGwPlaceholder {
					expIPAMConfig[i].Gateway = llAddr.String()
				}
			}
			assert.Check(t, is.DeepEqual(insp.IPAM.Config, expIPAMConfig))
		})
	})
}

func newHostInL3Seg(t *testing.T, name, ip4, ip6 string) (networking.Host, func()) {
	// Set up a netns for each test to avoid sysctl and iptables pollution.
	addr4 := netip.MustParseAddr(ip4)
	addr6 := netip.MustParseAddr(ip6)
	l3 := networking.NewL3Segment(t, "test-"+name,
		netip.PrefixFrom(addr4, 24),
		netip.PrefixFrom(addr6, 64),
	)
	hostname := "host-" + name
	l3.AddHost(t, hostname, "ns-"+name, "eth0",
		netip.PrefixFrom(addr4.Next(), 24),
		netip.PrefixFrom(addr6.Next(), 64),
	)
	return l3.Hosts[hostname], func() { l3.Destroy(t) }
}

// createBridge creates a bridge device named ifName, brings it up, waits for
// the kernel to assign a link-local IPv6 address, assigns addrs, and returns
// the kernel-assigned LL address.
func createBridge(t *testing.T, ifName string, addrs []string) net.IP {
	t.Helper()

	// Get a netlink handle in this netns.
	nlh, err := nlwrap.NewHandle()
	assert.NilError(t, err)
	defer nlh.Close()

	link := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: ifName,
		},
	}
	err = nlh.LinkAdd(link)
	assert.NilError(t, err)

	// Bring the interface up, and wait for the kernel to assign its link-local
	// address (to cause maximum confusion - the LL address shouldn't be selected
	// as "bip6").
	brLink, err := nlh.LinkByName(ifName)
	assert.NilError(t, err)
	err = nlh.LinkSetUp(brLink)
	assert.NilError(t, err)
	var llAddr net.IP
	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		addrs, err := nlh.AddrList(brLink, netlink.FAMILY_V6)
		if err != nil {
			return poll.Error(err)
		}
		if len(addrs) == 0 {
			return poll.Continue("no IPv6 addresses")
		}
		llAddr = addrs[0].IP
		return poll.Success()
	})

	for _, addr := range addrs {
		ip, ipNet, err := net.ParseCIDR(addr)
		assert.NilError(t, err)
		ipNet.IP = ip
		err = nlh.AddrAdd(link, &netlink.Addr{IPNet: ipNet})
		assert.NilError(t, err)
	}
	return llAddr
}
