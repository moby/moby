package networking

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	n "github.com/docker/docker/integration/network"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestBridgeICC tries to ping container ctr1 from container ctr2 using its hostname. Thus, this test checks:
// 1. DNS resolution ; 2. ARP/NDP ; 3. whether containers can communicate with each other ; 4. kernel-assigned SLAAC
// addresses.
func TestBridgeICC(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name           string
		bridgeOpts     []func(*networktypes.CreateOptions)
		ctr1MacAddress string
		isIPv6         bool
		isLinkLocal    bool
		pingHost       string
	}{
		{
			name:       "IPv4 non-internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){},
		},
		{
			name: "IPv4 internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithInternal(),
			},
		},
		{
			name: "IPv6 ULA on non-internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 ULA on internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithInternal(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 link-local address on non-internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				// There's no real way to specify an IPv6 network is only used with SLAAC link-local IPv6 addresses.
				// What we can do instead, is to tell the IPAM driver to assign addresses from the link-local prefix.
				// Each container will have two link-local addresses: 1. a SLAAC address assigned by the kernel ;
				// 2. the one dynamically assigned by the IPAM driver.
				network.WithIPAM("fe80::/64", "fe80::1"),
			},
			isLinkLocal: true,
			isIPv6:      true,
		},
		{
			name: "IPv6 link-local address on internal network",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithInternal(),
				// See the note above about link-local addresses.
				network.WithIPAM("fe80::/64", "fe80::1"),
			},
			isLinkLocal: true,
			isIPv6:      true,
		},
		{
			// As for 'LL non-internal', ping the container by name instead of by address
			// - the busybox test containers only have one interface with a link local
			// address, so the zone index is not required:
			//   RFC-4007, section 6: "[...] for nodes with only a single non-loopback
			//   interface (e.g., a single Ethernet interface), the common case, link-local
			//   addresses need not be qualified with a zone index."
			// So, for this common case, LL addresses should be included in DNS config.
			name: "IPv6 link-local address on non-internal network ping by name",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fe80::/64", "fe80::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 nonstandard link-local subnet on non-internal network ping by name",
			// No interfaces apart from the one on the bridge network with this non-default
			// subnet will be on this link local subnet (it's not currently possible to
			// configure two networks with the same LL subnet, although perhaps it should
			// be). So, again, no zone index is required and the LL address should be
			// included in DNS config.
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fe80:1234::/64", "fe80:1234::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 non-internal network with SLAAC LL address",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			// Link-local address is derived from the MAC address, so we need to
			// specify one here to hardcode the SLAAC LL address below.
			ctr1MacAddress: "02:42:ac:11:00:02",
			pingHost:       "fe80::42:acff:fe11:2%eth0",
			isIPv6:         true,
		},
		{
			name: "IPv6 internal network with SLAAC LL address",
			bridgeOpts: []func(*networktypes.CreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			// Link-local address is derived from the MAC address, so we need to
			// specify one here to hardcode the SLAAC LL address below.
			ctr1MacAddress: "02:42:ac:11:00:02",
			pingHost:       "fe80::42:acff:fe11:2%eth0",
			isIPv6:         true,
		},
	}

	for tcID, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			bridgeName := fmt.Sprintf("testnet-icc-%d", tcID)
			network.CreateNoError(ctx, t, c, bridgeName, append(tc.bridgeOpts,
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridgeName))...)
			defer network.RemoveNoError(ctx, t, c, bridgeName)

			ctr1Name := fmt.Sprintf("ctr-icc-%d-1", tcID)
			var ctr1Opts []func(config *container.TestContainerConfig)
			if tc.ctr1MacAddress != "" {
				ctr1Opts = append(ctr1Opts, container.WithMacAddress(bridgeName, tc.ctr1MacAddress))
			}
			id1 := container.Run(ctx, t, c, append(ctr1Opts,
				container.WithName(ctr1Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("top"),
				container.WithNetworkMode(bridgeName))...)
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{
				Force: true,
			})

			pingHost := tc.pingHost
			if pingHost == "" {
				if tc.isLinkLocal {
					inspect := container.Inspect(ctx, t, c, id1)
					pingHost = inspect.NetworkSettings.Networks[bridgeName].GlobalIPv6Address + "%eth0"
				} else {
					pingHost = ctr1Name
				}
			}

			ipv := "-4"
			if tc.isIPv6 {
				ipv = "-6"
			}

			pingCmd := []string{"ping", "-c1", "-W3", ipv, pingHost}

			ctr2Name := fmt.Sprintf("ctr-icc-%d-2", tcID)
			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(attachCtx, t, c,
				container.WithName(ctr2Name),
				container.WithImage("busybox:latest"),
				container.WithCmd(pingCmd...),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{
				Force: true,
			})

			assert.Check(t, is.Equal(res.ExitCode, 0))
			assert.Check(t, is.Equal(res.Stderr.Len(), 0))
			assert.Check(t, is.Contains(res.Stdout.String(), "1 packets transmitted, 1 packets received"))
		})
	}
}

// TestBridgeINC makes sure two containers on two different bridge networks can't communicate with each other.
func TestBridgeINC(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	type bridgesOpts struct {
		bridge1Opts []func(*networktypes.CreateOptions)
		bridge2Opts []func(*networktypes.CreateOptions)
	}

	testcases := []struct {
		name    string
		bridges bridgesOpts
		ipv6    bool
		stdout  string
		stderr  string
	}{
		{
			name: "IPv4 non-internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*networktypes.CreateOptions){},
				bridge2Opts: []func(*networktypes.CreateOptions){},
			},
			stdout: "1 packets transmitted, 0 packets received",
		},
		{
			name: "IPv4 internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*networktypes.CreateOptions){network.WithInternal()},
				bridge2Opts: []func(*networktypes.CreateOptions){network.WithInternal()},
			},
			stderr: "sendto: Network is unreachable",
		},
		{
			name: "IPv6 ULA on non-internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*networktypes.CreateOptions){
					network.WithIPv6(),
					network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
				},
				bridge2Opts: []func(*networktypes.CreateOptions){
					network.WithIPv6(),
					network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
				},
			},
			ipv6:   true,
			stdout: "1 packets transmitted, 0 packets received",
		},
		{
			name: "IPv6 ULA on internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*networktypes.CreateOptions){
					network.WithIPv6(),
					network.WithInternal(),
					network.WithIPAM("fdf1:a844:390c:b200::/64", "fdf1:a844:390c:b200::1"),
				},
				bridge2Opts: []func(*networktypes.CreateOptions){
					network.WithIPv6(),
					network.WithInternal(),
					network.WithIPAM("fdf1:a844:390c:b247::/64", "fdf1:a844:390c:b247::1"),
				},
			},
			ipv6:   true,
			stderr: "sendto: Network is unreachable",
		},
	}

	for tcID, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			bridge1 := fmt.Sprintf("testnet-inc-%d-1", tcID)
			bridge2 := fmt.Sprintf("testnet-inc-%d-2", tcID)

			network.CreateNoError(ctx, t, c, bridge1, append(tc.bridges.bridge1Opts,
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridge1))...)
			defer network.RemoveNoError(ctx, t, c, bridge1)
			network.CreateNoError(ctx, t, c, bridge2, append(tc.bridges.bridge2Opts,
				network.WithDriver("bridge"),
				network.WithOption("com.docker.network.bridge.name", bridge2))...)
			defer network.RemoveNoError(ctx, t, c, bridge2)

			ctr1Name := sanitizeCtrName(t.Name() + "-ctr1")
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithImage("busybox:latest"),
				container.WithCmd("top"),
				container.WithNetworkMode(bridge1))
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{
				Force: true,
			})

			ctr1Info := container.Inspect(ctx, t, c, id1)
			targetAddr := ctr1Info.NetworkSettings.Networks[bridge1].IPAddress
			if tc.ipv6 {
				targetAddr = ctr1Info.NetworkSettings.Networks[bridge1].GlobalIPv6Address
			}

			pingCmd := []string{"ping", "-c1", "-W3", targetAddr}

			ctr2Name := sanitizeCtrName(t.Name() + "-ctr2")
			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(attachCtx, t, c,
				container.WithName(ctr2Name),
				container.WithImage("busybox:latest"),
				container.WithCmd(pingCmd...),
				container.WithNetworkMode(bridge2))
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{
				Force: true,
			})

			assert.Check(t, res.ExitCode != 0, "ping unexpectedly succeeded")
			assert.Check(t, is.Contains(res.Stdout.String(), tc.stdout))
			assert.Check(t, is.Contains(res.Stderr.String(), tc.stderr))
		})
	}
}

func TestDefaultBridgeIPv6(t *testing.T) {
	ctx := setupTest(t)

	testcases := []struct {
		name          string
		fixed_cidr_v6 string
	}{
		{
			name:          "IPv6 ULA",
			fixed_cidr_v6: "fd00:1234::/64",
		},
		{
			name:          "IPv6 LLA only",
			fixed_cidr_v6: "fe80::/64",
		},
		{
			name:          "IPv6 nonstandard LLA only",
			fixed_cidr_v6: "fe80:1234::/64",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			d := daemon.New(t)
			d.StartWithBusybox(ctx, t,
				"--ipv6",
				"--fixed-cidr-v6", tc.fixed_cidr_v6,
			)
			defer d.Stop(t)

			c := d.NewClientT(t)
			defer c.Close()

			cID := container.Run(ctx, t, c,
				container.WithImage("busybox:latest"),
				container.WithCmd("top"),
			)
			defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{
				Force: true,
			})

			networkName := "bridge"
			inspect := container.Inspect(ctx, t, c, cID)
			pingHost := inspect.NetworkSettings.Networks[networkName].GlobalIPv6Address

			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(attachCtx, t, c,
				container.WithImage("busybox:latest"),
				container.WithCmd("ping", "-c1", "-W3", pingHost),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{
				Force: true,
			})

			assert.Check(t, is.Equal(res.ExitCode, 0))
			assert.Check(t, is.Equal(res.Stderr.String(), ""))
			assert.Check(t, is.Contains(res.Stdout.String(), "1 packets transmitted, 1 packets received"))
		})
	}
}

// Check that it's possible to change 'fixed-cidr-v6' and restart the daemon.
func TestDefaultBridgeAddresses(t *testing.T) {
	ctx := setupTest(t)

	type testStep struct {
		stepName    string
		fixedCIDRV6 string
		expAddrs    []string
	}

	testcases := []struct {
		name  string
		steps []testStep
	}{
		{
			name: "Unique-Local Subnet Changes",
			steps: []testStep{
				{
					stepName:    "Set up initial UL prefix",
					fixedCIDRV6: "fd1c:f1a0:5d8d:aaaa::/64",
					expAddrs:    []string{"fd1c:f1a0:5d8d:aaaa::1/64", "fe80::"},
				},
				{
					// Modify that prefix, the default bridge's address must be deleted and re-added.
					stepName:    "Modify UL prefix - address change",
					fixedCIDRV6: "fd1c:f1a0:5d8d:bbbb::/64",
					expAddrs:    []string{"fd1c:f1a0:5d8d:bbbb::1/64", "fe80::"},
				},
				{
					// Modify the prefix length, the default bridge's address should not change.
					stepName:    "Modify UL prefix - no address change",
					fixedCIDRV6: "fd1c:f1a0:5d8d:bbbb::/80",
					// The prefix length displayed by 'ip a' is not updated - it's informational, and
					// can't be changed without unnecessarily deleting and re-adding the address.
					expAddrs: []string{"fd1c:f1a0:5d8d:bbbb::1/64", "fe80::"},
				},
			},
		},
		{
			name: "Link-Local Subnet Changes",
			steps: []testStep{
				{
					stepName:    "Standard LL subnet prefix",
					fixedCIDRV6: "fe80::/64",
					expAddrs:    []string{"fe80::"},
				},
				{
					// Modify that prefix, the default bridge's address must be deleted and re-added.
					// The bridge must still have an address in the required (standard) LL subnet.
					stepName:    "Nonstandard LL prefix - address change",
					fixedCIDRV6: "fe80:1234::/32",
					expAddrs:    []string{"fe80:1234::1/32", "fe80::"},
				},
				{
					// Modify the prefix length, the addresses should not change.
					stepName:    "Modify LL prefix - no address change",
					fixedCIDRV6: "fe80:1234::/64",
					// The prefix length displayed by 'ip a' is not updated - it's informational, and
					// can't be changed without unnecessarily deleting and re-adding the address.
					expAddrs: []string{"fe80:1234::1/", "fe80::"},
				},
			},
		},
	}

	d := daemon.New(t)
	defer d.Stop(t)
	c := d.NewClientT(t)

	for _, tc := range testcases {
		for _, step := range tc.steps {
			t.Run(tc.name+"/"+step.stepName, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				// Check that the daemon starts - regression test for:
				//   https://github.com/moby/moby/issues/46829
				d.StartWithBusybox(ctx, t, "--ipv6", "--fixed-cidr-v6="+step.fixedCIDRV6)

				// Start a container, so that the bridge is set "up" and gets a kernel_ll address.
				cID := container.Run(ctx, t, c)
				defer c.ContainerRemove(ctx, cID, containertypes.RemoveOptions{Force: true})

				d.Stop(t)

				// Check that the expected addresses have been applied to the bridge. (Skip in
				// rootless mode, because the bridge is in a different network namespace.)
				if !testEnv.IsRootless() {
					res := testutil.RunCommand(ctx, "ip", "-6", "addr", "show", "docker0")
					assert.Equal(t, res.ExitCode, 0, step.stepName)
					stdout := res.Stdout()
					for _, expAddr := range step.expAddrs {
						assert.Check(t, is.Contains(stdout, expAddr))
					}
				}
			})
		}
	}
}

// Test that a container on an 'internal' network has IP connectivity with
// the host (on its own subnet, because the n/w bridge has an address on that
// subnet, and it's in the host's namespace).
// Regression test for https://github.com/moby/moby/issues/47329
func TestInternalNwConnectivity(t *testing.T) {
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const bridgeName = "intnw"
	const gw4 = "172.30.0.1"
	const gw6 = "fda9:4130:4715::1234"
	network.CreateNoError(ctx, t, c, bridgeName,
		network.WithInternal(),
		network.WithIPv6(),
		network.WithIPAM("172.30.0.0/24", gw4),
		network.WithIPAM("fda9:4130:4715::/64", gw6),
		network.WithDriver("bridge"),
		network.WithOption("com.docker.network.bridge.name", bridgeName),
	)
	defer network.RemoveNoError(ctx, t, c, bridgeName)

	const ctrName = "intctr"
	id := container.Run(ctx, t, c,
		container.WithName(ctrName),
		container.WithImage("busybox:latest"),
		container.WithCmd("top"),
		container.WithNetworkMode(bridgeName),
	)
	defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

	execCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	res := container.ExecT(execCtx, t, c, id, []string{"ping", "-c1", "-W3", gw4})
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Equal(res.Stderr(), ""))
	assert.Check(t, is.Contains(res.Stdout(), "1 packets transmitted, 1 packets received"))

	res = container.ExecT(execCtx, t, c, id, []string{"ping6", "-c1", "-W3", gw6})
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Equal(res.Stderr(), ""))
	assert.Check(t, is.Contains(res.Stdout(), "1 packets transmitted, 1 packets received"))

	// Addresses outside the internal subnet must not be accessible.
	res = container.ExecT(execCtx, t, c, id, []string{"ping", "-c1", "-W3", "8.8.8.8"})
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Contains(res.Stderr(), "Network is unreachable"))
}

// Check that the container's interfaces have no IPv6 address when IPv6 is
// disabled in a container via sysctl (including 'lo').
func TestDisableIPv6Addrs(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name    string
		sysctls map[string]string
		expIPv6 bool
	}{
		{
			name:    "IPv6 enabled",
			expIPv6: true,
		},
		{
			name:    "IPv6 disabled",
			sysctls: map[string]string{"net.ipv6.conf.all.disable_ipv6": "1"},
		},
	}

	const netName = "testnet"
	network.CreateNoError(ctx, t, c, netName,
		network.WithIPv6(),
		network.WithIPAM("fda0:ef3d:6430:abcd::/64", "fda0:ef3d:6430:abcd::1"),
	)
	defer network.RemoveNoError(ctx, t, c, netName)

	inet6RE := regexp.MustCompile(`inet6[ \t]+[0-9a-f:]*`)

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			opts := []func(config *container.TestContainerConfig){
				container.WithCmd("ip", "a"),
				container.WithNetworkMode(netName),
			}
			if len(tc.sysctls) > 0 {
				opts = append(opts, container.WithSysctls(tc.sysctls))
			}

			runRes := container.RunAttach(ctx, t, c, opts...)
			defer c.ContainerRemove(ctx, runRes.ContainerID,
				containertypes.RemoveOptions{Force: true},
			)

			stdout := runRes.Stdout.String()
			inet6 := inet6RE.FindAllString(stdout, -1)
			if tc.expIPv6 {
				assert.Check(t, len(inet6) > 0, "Expected IPv6 addresses but found none.")
			} else {
				assert.Check(t, is.DeepEqual(inet6, []string{}, cmpopts.EquateEmpty()))
			}
		})
	}
}

// Check that a container in a network with IPv4 disabled doesn't get
// IPv4 addresses.
func TestDisableIPv4(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t, daemon.WithExperimental())
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	testcases := []struct {
		name       string
		apiVersion string
		expIPv4    bool
	}{
		{
			name:    "disable ipv4",
			expIPv4: false,
		},
		{
			name:       "old api ipv4 not disabled",
			apiVersion: "1.46",
			expIPv4:    true,
		},
	}

	for _, tc := range testcases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			c := d.NewClientT(t, client.WithVersion(tc.apiVersion))

			const netName = "testnet"
			network.CreateNoError(ctx, t, c, netName,
				network.WithIPv4(false),
				network.WithIPv6(),
			)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := container.Run(ctx, t, c, container.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

			loRes := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "lo"})
			assert.Check(t, is.Contains(loRes.Combined(), " inet ")) // 127.0.0.1
			assert.Check(t, is.Contains(loRes.Combined(), " inet6 "))

			eth0Res := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "eth0"})
			if tc.expIPv4 {
				assert.Check(t, is.Contains(eth0Res.Combined(), " inet "))
			} else {
				assert.Check(t, !strings.Contains(eth0Res.Combined(), " inet "),
					"result.Combined(): %s", eth0Res.Combined())
			}
			assert.Check(t, is.Contains(eth0Res.Combined(), " inet6 "))
		})
	}
}

// Check that an interface to an '--ipv6=false' network has no IPv6
// address - either IPAM assigned, or kernel-assigned LL, but the loopback
// interface does still have an IPv6 address ('::1').
func TestNonIPv6Network(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "testnet"
	network.CreateNoError(ctx, t, c, netName)
	defer network.RemoveNoError(ctx, t, c, netName)

	id := container.Run(ctx, t, c, container.WithNetworkMode(netName))
	defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

	loRes := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "lo"})
	assert.Check(t, is.Contains(loRes.Combined(), " inet "))
	assert.Check(t, is.Contains(loRes.Combined(), " inet6 "))

	eth0Res := container.ExecT(ctx, t, c, id, []string{"ip", "a", "show", "dev", "eth0"})
	assert.Check(t, is.Contains(eth0Res.Combined(), " inet "))
	assert.Check(t, !strings.Contains(eth0Res.Combined(), " inet6 "),
		"result.Combined(): %s", eth0Res.Combined())

	sysctlRes := container.ExecT(ctx, t, c, id, []string{"sysctl", "-n", "net.ipv6.conf.eth0.disable_ipv6"})
	assert.Check(t, is.Equal(strings.TrimSpace(sysctlRes.Combined()), "1"))
}

// Check that starting the daemon with '--ip6tables=false' means no ip6tables
// rules get set up for an IPv6 bridge network.
func TestNoIP6Tables(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)

	testcases := []struct {
		name        string
		option      string
		expIPTables bool
	}{
		{
			name:        "ip6tables on",
			option:      "--ip6tables=true",
			expIPTables: true,
		},
		{
			name:   "ip6tables off",
			option: "--ip6tables=false",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			d := daemon.New(t)
			d.StartWithBusybox(ctx, t, tc.option)
			defer d.Stop(t)

			c := d.NewClientT(t)
			defer c.Close()

			const netName = "testnet"
			const bridgeName = "testbr"
			const subnet = "fdb3:2511:e851:34a9::/64"
			network.CreateNoError(ctx, t, c, netName,
				network.WithIPv6(),
				network.WithOption("com.docker.network.bridge.name", bridgeName),
				network.WithIPAM(subnet, "fdb3:2511:e851:34a9::1"),
			)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := container.Run(ctx, t, c, container.WithNetworkMode(netName))
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

			res, err := exec.Command("/usr/sbin/ip6tables-save").CombinedOutput()
			assert.NilError(t, err)
			if tc.expIPTables {
				assert.Check(t, is.Contains(string(res), subnet))
				assert.Check(t, is.Contains(string(res), bridgeName))
			} else {
				assert.Check(t, !strings.Contains(string(res), subnet),
					fmt.Sprintf("Didn't expect to find '%s' in '%s'", subnet, string(res)))
				assert.Check(t, !strings.Contains(string(res), bridgeName),
					fmt.Sprintf("Didn't expect to find '%s' in '%s'", bridgeName, string(res)))
			}
		})
	}
}

// Test that it's possible to set a sysctl on an interface in the container
// when using API 1.46 (in later versions of the API, per-interface sysctls
// must be set using driver option 'com.docker.network.endpoint.sysctls').
// Regression test for https://github.com/moby/moby/issues/47619
func TestSetInterfaceSysctl(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t, client.WithVersion("1.46"))
	defer c.Close()

	const scName = "net.ipv4.conf.eth0.forwarding"
	opts := []func(config *container.TestContainerConfig){
		container.WithCmd("sysctl", scName),
		container.WithSysctls(map[string]string{scName: "1"}),
	}

	runRes := container.RunAttach(ctx, t, c, opts...)
	defer c.ContainerRemove(ctx, runRes.ContainerID,
		containertypes.RemoveOptions{Force: true},
	)

	stdout := runRes.Stdout.String()
	assert.Check(t, is.Contains(stdout, scName))
}

// With a read-only "/proc/sys/net" filesystem (simulated using env var
// DOCKER_TEST_RO_DISABLE_IPV6), check that if IPv6 can't be disabled on a
// container interface, container creation fails.
// Regression test for https://github.com/moby/moby/issues/47751
func TestReadOnlySlashProc(t *testing.T) {
	ctx := setupTest(t)

	testcases := []struct {
		name      string
		daemonEnv []string
		expErr    string
	}{
		{
			name: "Normality",
		},
		{
			name: "Read only",
			daemonEnv: []string{
				"DOCKER_TEST_RO_DISABLE_IPV6=1",
			},
			expErr: "failed to disable IPv6 on container's interface eth0",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			d := daemon.New(t, daemon.WithEnvVars(tc.daemonEnv...))
			d.StartWithBusybox(ctx, t)
			defer d.Stop(t)
			c := d.NewClientT(t)

			const net4Name = "testnet4"
			network.CreateNoError(ctx, t, c, net4Name)
			defer network.RemoveNoError(ctx, t, c, net4Name)
			id4 := container.Create(ctx, t, c,
				container.WithNetworkMode(net4Name),
				container.WithCmd("ls"),
			)
			defer c.ContainerRemove(ctx, id4, containertypes.RemoveOptions{Force: true})
			err := c.ContainerStart(ctx, id4, containertypes.StartOptions{})
			if tc.expErr == "" {
				assert.Check(t, err)
			} else {
				assert.Check(t, is.ErrorContains(err, tc.expErr))
			}

			// It should always be possible to create a container on an IPv6 network (IPv6
			// doesn't need to be disabled on the interface).
			const net6Name = "testnet6"
			network.CreateNoError(ctx, t, c, net6Name,
				network.WithIPv6(),
				network.WithIPAM("fd5c:15e3:0b62:5395::/64", "fd5c:15e3:0b62:5395::1"),
			)
			defer network.RemoveNoError(ctx, t, c, net6Name)
			id6 := container.Run(ctx, t, c,
				container.WithNetworkMode(net6Name),
				container.WithCmd("ls"),
			)
			defer c.ContainerRemove(ctx, id6, containertypes.RemoveOptions{Force: true})
		})
	}
}

// Test that it's possible to set a sysctl on an interface in the container
// using DriverOpts.
func TestSetEndpointSysctl(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const scName = "net.ipv4.conf.eth0.forwarding"
	for _, ifname := range []string{"IFNAME", "ifname"} {
		for _, val := range []string{"0", "1"} {
			t.Run("ifname="+ifname+"/val="+val, func(t *testing.T) {
				ctx := testutil.StartSpan(ctx, t)
				runRes := container.RunAttach(ctx, t, c,
					container.WithCmd("sysctl", "-qn", scName),
					container.WithEndpointSettings(networktypes.NetworkBridge, &networktypes.EndpointSettings{
						DriverOpts: map[string]string{
							netlabel.EndpointSysctls: "net.ipv4.conf." + ifname + ".forwarding=" + val,
						},
					}),
				)
				defer c.ContainerRemove(ctx, runRes.ContainerID, containertypes.RemoveOptions{Force: true})

				stdout := runRes.Stdout.String()
				assert.Check(t, is.Equal(strings.TrimSpace(stdout), val))
			})
		}
	}
}

// TestContainerDisabledIPv6 checks that a container with IPv6 disabled does not
// get an IPv6 address when joining an IPv6 network. (TestDisableIPv6Addrs checks
// that no IPv6 addresses assigned to interfaces, this test checks that there are
// no IPv6 records in the DNS and that the IPv4 DNS response is correct.)
func TestContainerDisabledIPv6(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "ipv6br"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, netName),
		network.WithIPv6(),
		network.WithIPAM("fd64:40cd:7fb4:8971::/64", "fd64:40cd:7fb4:8971::1"),
	)
	defer network.RemoveNoError(ctx, t, c, netName)

	// Run a container with IPv6 enabled.
	ctrWith6 := container.Run(ctx, t, c,
		container.WithNetworkMode(netName),
	)
	defer c.ContainerRemove(ctx, ctrWith6, containertypes.RemoveOptions{Force: true})
	inspect := container.Inspect(ctx, t, c, ctrWith6)
	addr := inspect.NetworkSettings.Networks[netName].GlobalIPv6Address
	assert.Check(t, is.Contains(addr, "fd64:40cd:7fb4:8971"))

	// Run a container with IPv6 disabled.
	const ctrNo6Name = "ctrNo6"
	ctrNo6 := container.Run(ctx, t, c,
		container.WithName(ctrNo6Name),
		container.WithNetworkMode(netName),
		container.WithSysctls(map[string]string{"net.ipv6.conf.all.disable_ipv6": "1"}),
	)
	defer c.ContainerRemove(ctx, ctrNo6, containertypes.RemoveOptions{Force: true})
	inspect = container.Inspect(ctx, t, c, ctrNo6)
	addr = inspect.NetworkSettings.Networks[netName].GlobalIPv6Address
	assert.Check(t, is.Equal(addr, ""))

	execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Check that the with-IPv6 container can ping the other using its IPv4 address.
	res := container.ExecT(execCtx, t, c, ctrWith6, []string{"ping", "-4", "-c1", "-W3", ctrNo6Name})
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Contains(res.Stdout(), "1 packets transmitted, 1 packets received"))
	assert.Check(t, is.Equal(res.Stderr(), ""))

	// Check that the with-IPv6 container doesn't find an IPv6 address for the other
	// (fail fast on the address lookup, rather than timing out on the ping).
	res = container.ExecT(execCtx, t, c, ctrWith6, []string{"ping", "-6", "-c1", "-W3", ctrNo6Name})
	assert.Check(t, is.Equal(res.ExitCode, 1))
	assert.Check(t, is.Equal(res.Stdout(), ""))
	assert.Check(t, is.Contains(res.Stderr(), "bad address"))
}

type expProxyCfg struct {
	proto      string
	hostIP     string
	hostPort   string
	ctrName    string
	ctrNetName string
	ctrIPv4    bool
	ctrPort    string
}

func TestGatewaySelection(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "proxies run in child namespace")

	ctx := setupTest(t)
	d := daemon.New(t, daemon.WithExperimental())
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	const netName4 = "net4"
	network.CreateNoError(ctx, t, c, netName4)
	defer network.RemoveNoError(ctx, t, c, netName4)

	const netName6 = "net6"
	netId6 := network.CreateNoError(ctx, t, c, netName6, network.WithIPv6(), network.WithIPv4(false))
	defer network.RemoveNoError(ctx, t, c, netName6)

	const netName46 = "net46"
	netId46 := network.CreateNoError(ctx, t, c, netName46, network.WithIPv6())
	defer network.RemoveNoError(ctx, t, c, netName46)

	master := "dm-dummy0"
	n.CreateMasterDummy(ctx, t, master)
	defer n.DeleteInterface(ctx, t, master)
	const netNameIpvlan6 = "ipvlan6"
	netIdIpvlan6 := network.CreateNoError(ctx, t, c, netNameIpvlan6,
		network.WithIPvlan("dm-dummy0", "l2"),
		network.WithIPv4(false),
		network.WithIPv6(),
	)
	defer network.RemoveNoError(ctx, t, c, netNameIpvlan6)

	const ctrName = "ctr"
	ctrId := container.Run(ctx, t, c,
		container.WithName(ctrName),
		container.WithNetworkMode(netName4),
		container.WithExposedPorts("80"),
		container.WithPortMap(nat.PortMap{"80": {{HostPort: "8080"}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})

	// The container only has an IPv4 endpoint, it should be the gateway, and
	// the host-IPv6 should be proxied to container-IPv4.
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName4, true, "80"},
		{"tcp", "::", "8080", ctrName, netName4, true, "80"},
	})

	// Connect the IPv6-only network. The IPv6 endpoint should become the
	// gateway for IPv6, the IPv4 endpoint should be reconfigured as the
	// gateway for IPv4 only.
	err := c.NetworkConnect(ctx, netId6, ctrId, nil)
	assert.NilError(t, err)
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName4, true, "80"},
		{"tcp", "::", "8080", ctrName, netName6, false, "80"},
	})

	// Disconnect the IPv6-only network, the IPv4 should get back the mapping
	// from host-IPv6.
	err = c.NetworkDisconnect(ctx, netId6, ctrId, false)
	assert.NilError(t, err)
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName4, true, "80"},
		{"tcp", "::", "8080", ctrName, netName4, true, "80"},
	})

	// Connect the dual-stack network, it should become the gateway for v6 and v4.
	err = c.NetworkConnect(ctx, netId46, ctrId, nil)
	assert.NilError(t, err)
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName46, true, "80"},
		{"tcp", "::", "8080", ctrName, netName46, false, "80"},
	})

	// Go back to the IPv4-only gateway, with proxy from host IPv6.
	err = c.NetworkDisconnect(ctx, netId46, ctrId, false)
	assert.NilError(t, err)
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName4, true, "80"},
		{"tcp", "::", "8080", ctrName, netName4, true, "80"},
	})

	// Connect the IPv6-only ipvlan network, its new Endpoint should become the IPv6
	// gateway, so the IPv4-only bridge is expected to drop its mapping from host IPv6.
	err = c.NetworkConnect(ctx, netIdIpvlan6, ctrId, nil)
	assert.NilError(t, err)
	checkProxies(ctx, t, c, d.Pid(), []expProxyCfg{
		{"tcp", "0.0.0.0", "8080", ctrName, netName4, true, "80"},
	})
}

func checkProxies(ctx context.Context, t *testing.T, c *client.Client, daemonPid int, exp []expProxyCfg) {
	t.Helper()
	makeExpStr := func(proto, hostIP, hostPort, ctrIP, ctrPort string) string {
		return fmt.Sprintf("%s:%s/%s <-> %s:%s", hostIP, hostPort, proto, ctrIP, ctrPort)
	}

	wantProxies := make([]string, len(exp))
	for _, e := range exp {
		inspect := container.Inspect(ctx, t, c, e.ctrName)
		nw := inspect.NetworkSettings.Networks[e.ctrNetName]
		ctrIP := nw.GlobalIPv6Address
		if e.ctrIPv4 {
			ctrIP = nw.IPAddress
		}
		wantProxies = append(wantProxies, makeExpStr(e.proto, e.hostIP, e.hostPort, ctrIP, e.ctrPort))
	}

	gotProxies := make([]string, len(exp))
	res, err := exec.Command("ps", "-f", "--ppid", strconv.Itoa(daemonPid)).CombinedOutput()
	assert.NilError(t, err)
	for _, line := range strings.Split(string(res), "\n") {
		_, args, ok := strings.Cut(line, "docker-proxy")
		if !ok {
			continue
		}
		var proto, hostIP, hostPort, ctrIP, ctrPort string
		var useListenFd bool
		fs := flag.NewFlagSet("docker-proxy", flag.ContinueOnError)
		fs.StringVar(&proto, "proto", "", "Protocol")
		fs.StringVar(&hostIP, "host-ip", "", "Host IP")
		fs.StringVar(&hostPort, "host-port", "", "Host Port")
		fs.StringVar(&ctrIP, "container-ip", "", "Container IP")
		fs.StringVar(&ctrPort, "container-port", "", "Container Port")
		fs.BoolVar(&useListenFd, "use-listen-fd", false, "Use listen fd")
		fs.Parse(strings.Split(strings.TrimSpace(args), " "))
		gotProxies = append(gotProxies, makeExpStr(proto, hostIP, hostPort, ctrIP, ctrPort))
	}

	assert.DeepEqual(t, gotProxies, wantProxies)
}
