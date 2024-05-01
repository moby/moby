package networking

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/google/go-cmp/cmp/cmpopts"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

// TestBridgeICC tries to ping container ctr1 from container ctr2 using its hostname. Thus, this test checks:
// 1. DNS resolution ; 2. ARP/NDP ; 3. whether containers can communicate with each other ; 4. kernel-assigned SLAAC
// addresses.
func TestBridgeICC(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name           string
		bridgeOpts     []func(*types.NetworkCreate)
		ctr1MacAddress string
		isIPv6         bool
		isLinkLocal    bool
		pingHost       string
	}{
		{
			name:       "IPv4 non-internal network",
			bridgeOpts: []func(*types.NetworkCreate){},
		},
		{
			name: "IPv4 internal network",
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithInternal(),
			},
		},
		{
			name: "IPv6 ULA on non-internal network",
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 ULA on internal network",
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithInternal(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 link-local address on non-internal network",
			bridgeOpts: []func(*types.NetworkCreate){
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
			bridgeOpts: []func(*types.NetworkCreate){
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
			bridgeOpts: []func(*types.NetworkCreate){
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
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fe80:1234::/64", "fe80:1234::1"),
			},
			isIPv6: true,
		},
		{
			name: "IPv6 non-internal network with SLAAC LL address",
			bridgeOpts: []func(*types.NetworkCreate){
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
			bridgeOpts: []func(*types.NetworkCreate){
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

// TestBridgeICCWindows tries to ping container ctr1 from container ctr2 using its hostname.
// Checks DNS resolution, and whether containers can communicate with each other.
// Regression test for https://github.com/moby/moby/issues/47370
func TestBridgeICCWindows(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType != "windows")

	ctx := setupTest(t)
	c := testEnv.APIClient()

	testcases := []struct {
		name    string
		netName string
	}{
		{
			name:    "Default nat network",
			netName: "nat",
		},
		{
			name:    "User defined nat network",
			netName: "mynat",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			if tc.netName != "nat" {
				network.CreateNoError(ctx, t, c, tc.netName,
					network.WithDriver("nat"),
				)
				defer network.RemoveNoError(ctx, t, c, tc.netName)
			}

			const ctr1Name = "ctr1"
			id1 := container.Run(ctx, t, c,
				container.WithName(ctr1Name),
				container.WithNetworkMode(tc.netName),
			)
			defer c.ContainerRemove(ctx, id1, containertypes.RemoveOptions{Force: true})

			pingCmd := []string{"ping", "-n", "1", "-w", "3000", ctr1Name}

			const ctr2Name = "ctr2"
			attachCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(attachCtx, t, c,
				container.WithName(ctr2Name),
				container.WithCmd(pingCmd...),
				container.WithNetworkMode(tc.netName),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})

			assert.Check(t, is.Equal(res.ExitCode, 0))
			assert.Check(t, is.Equal(res.Stderr.Len(), 0))
			assert.Check(t, is.Contains(res.Stdout.String(), "Sent = 1, Received = 1, Lost = 0"))
		})
	}
}

// TestBridgeINC makes sure two containers on two different bridge networks can't communicate with each other.
func TestBridgeINC(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	type bridgesOpts struct {
		bridge1Opts []func(*types.NetworkCreate)
		bridge2Opts []func(*types.NetworkCreate)
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
				bridge1Opts: []func(*types.NetworkCreate){},
				bridge2Opts: []func(*types.NetworkCreate){},
			},
			stdout: "1 packets transmitted, 0 packets received",
		},
		{
			name: "IPv4 internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*types.NetworkCreate){network.WithInternal()},
				bridge2Opts: []func(*types.NetworkCreate){network.WithInternal()},
			},
			stderr: "sendto: Network is unreachable",
		},
		{
			name: "IPv6 ULA on non-internal network",
			bridges: bridgesOpts{
				bridge1Opts: []func(*types.NetworkCreate){
					network.WithIPv6(),
					network.WithIPAM("fdf1:a844:380c:b200::/64", "fdf1:a844:380c:b200::1"),
				},
				bridge2Opts: []func(*types.NetworkCreate){
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
				bridge1Opts: []func(*types.NetworkCreate){
					network.WithIPv6(),
					network.WithInternal(),
					network.WithIPAM("fdf1:a844:390c:b200::/64", "fdf1:a844:390c:b200::1"),
				},
				bridge2Opts: []func(*types.NetworkCreate){
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

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
				"--experimental",
				"--ip6tables",
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

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

	for _, preserveKernelLL := range []bool{false, true} {
		var dopts []daemon.Option
		if preserveKernelLL {
			dopts = append(dopts, daemon.WithEnvVars("DOCKER_BRIDGE_PRESERVE_KERNEL_LL=1"))
		}
		d := daemon.New(t, dopts...)
		c := d.NewClientT(t)

		for _, tc := range testcases {
			for _, step := range tc.steps {
				tcName := fmt.Sprintf("kernel_ll_%v/%s/%s", preserveKernelLL, tc.name, step.stepName)
				t.Run(tcName, func(t *testing.T) {
					ctx := testutil.StartSpan(ctx, t)
					// Check that the daemon starts - regression test for:
					//   https://github.com/moby/moby/issues/46829
					d.StartWithBusybox(ctx, t, "--experimental", "--ipv6", "--ip6tables", "--fixed-cidr-v6="+step.fixedCIDRV6)

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
}

// Test that a container on an 'internal' network has IP connectivity with
// the host (on its own subnet, because the n/w bridge has an address on that
// subnet, and it's in the host's namespace).
// Regression test for https://github.com/moby/moby/issues/47329
func TestInternalNwConnectivity(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--experimental", "--ip6tables")
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
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

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

// Check that an interface to an '--ipv6=false' network has no IPv6
// address - either IPAM assigned, or kernel-assigned LL, but the loopback
// interface does still have an IPv6 address ('::1').
func TestNonIPv6Network(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

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

// Test that it's possible to set a sysctl on an interface in the container.
// Regression test for https://github.com/moby/moby/issues/47619
func TestSetInterfaceSysctl(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "no sysctl on Windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
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
// container interface, container creation fails - unless the error is ignored by
// setting env var DOCKER_ALLOW_IPV6_ON_IPV4_INTERFACE=1.
// Regression test for https://github.com/moby/moby/issues/47751
func TestReadOnlySlashProc(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

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
			name: "Read only no workaround",
			daemonEnv: []string{
				"DOCKER_TEST_RO_DISABLE_IPV6=1",
			},
			expErr: "failed to disable IPv6 on container's interface eth0, set env var DOCKER_ALLOW_IPV6_ON_IPV4_INTERFACE=1 to ignore this error",
		},
		{
			name: "Read only with workaround",
			daemonEnv: []string{
				"DOCKER_TEST_RO_DISABLE_IPV6=1",
				"DOCKER_ALLOW_IPV6_ON_IPV4_INTERFACE=1",
			},
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

// Check that masquerading can be disabled for IPv4/IPv6/both on a bridge
// network, and that the choice is preserved over a daemon restart.
func TestDisableMasquerade(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "no iptables on Windows")
	skip.If(t, testEnv.IsRootless(), "can't see iptables in rootless mode")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name string
		dOpt string
		exp4 bool
		exp6 bool
	}{
		{
			name: "default",
			exp4: true,
			exp6: true,
		},
		{
			name: "disable both",
			dOpt: "com.docker.network.bridge.enable_ip_masquerade",
		},
		{
			name: "disable 4",
			dOpt: "com.docker.network.bridge.enable_ip4_masquerade",
			exp6: true,
		},
		{
			name: "disable 6",
			dOpt: "com.docker.network.bridge.enable_ip6_masquerade",
			exp4: true,
		},
	}

	// Generate test config - a bridge network per test, subnet number is the test's index.
	type ipam struct {
		netName      string
		subnet4, gw4 string
		subnet6, gw6 string
	}
	td := make([]*ipam, len(testcases))
	for i := range testcases {
		td[i] = &ipam{
			netName: fmt.Sprintf("testnet%d", i),
			subnet4: fmt.Sprintf("172.24.%d.0/24", i),
			gw4:     fmt.Sprintf("172.24.%d.1", i),
			subnet6: fmt.Sprintf("fdc3:8049:23e9:%d::/64", i),
			gw6:     fmt.Sprintf("fdc3:8049:23e9:%d::1", i),
		}
	}

	// Create networks.
	for i, tc := range testcases {
		nOpts := []func(*types.NetworkCreate){
			network.WithIPv6(),
			network.WithIPAM(td[i].subnet4, td[i].gw4),
			network.WithIPAM(td[i].subnet6, td[i].gw6),
			network.WithOption(bridge.BridgeName, td[i].netName),
		}
		if tc.dOpt != "" {
			nOpts = append(nOpts, network.WithOption(tc.dOpt, "0"))
		}
		network.CreateNoError(ctx, t, c, td[i].netName, nOpts...)
		defer network.RemoveNoError(ctx, t, c, td[i].netName)
	}

	// Function to check for expected presence/absence of masquerade rules for each network.
	checkAllNetworks := func(when string) {
		for i, tc := range testcases {
			t.Run(when+"/"+tc.name, func(t *testing.T) {
				checkRule := func(cmd, netName, subnet, ipv string, expRule bool) {
					t.Helper()
					args := []string{
						"-t", "nat", "-C",
						"POSTROUTING", "-s", subnet, "!", "-o", netName, "-j", "MASQUERADE",
					}
					err := exec.Command(cmd, args...).Run()
					if expRule {
						assert.Check(t, err, ipv+" masquerade rule expected: "+strings.Join(args, " "))
					} else {
						var ee *exec.ExitError
						assert.Check(t, errors.As(err, &ee) && ee.ExitCode() == 1,
							ipv+" masquerade rule unexpected: "+strings.Join(args, " "))
					}
				}
				checkRule("iptables", td[i].netName, td[i].subnet4, "IPv4", tc.exp4)
				checkRule("ip6tables", td[i].netName, td[i].subnet6, "IPv6", tc.exp6)
			})
		}
	}

	checkAllNetworks("initial")
	d.Restart(t, "--experimental", "--ip6tables")
	checkAllNetworks("restarted")
}
