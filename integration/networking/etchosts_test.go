package networking

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/skip"
)

// Check that the '/etc/hosts' file in a container is created according to
// whether the container supports IPv6.
// Regression test for https://github.com/moby/moby/issues/35954
func TestEtcHostsIpv6(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t,
		"--ipv6",
		"--fixed-cidr-v6=fdc8:ffe2:d8d7:1234::/64")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name           string
		sysctls        map[string]string
		expIPv6Enabled bool
		expEtcHosts    string
	}{
		{
			// Create a container with no overrides, on the IPv6-enabled default bridge.
			// Expect the container to have a working '::1' address, on the assumption
			// the test host's kernel supports IPv6 - and for its '/etc/hosts' file to
			// include IPv6 addresses.
			name:           "IPv6 enabled",
			expIPv6Enabled: true,
			expEtcHosts: `127.0.0.1	localhost
::1	localhost ip6-localhost ip6-loopback
fe00::	ip6-localnet
ff00::	ip6-mcastprefix
ff02::1	ip6-allnodes
ff02::2	ip6-allrouters
`,
		},
		{
			// Create a container in the same network, with IPv6 disabled. Expect '::1'
			// not to be pingable, and no IPv6 addresses in its '/etc/hosts'.
			name:           "IPv6 disabled",
			sysctls:        map[string]string{"net.ipv6.conf.all.disable_ipv6": "1"},
			expIPv6Enabled: false,
			expEtcHosts:    "127.0.0.1\tlocalhost\n",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)
			ctrId := container.Run(ctx, t, c,
				container.WithName("etchosts_"+sanitizeCtrName(t.Name())),
				container.WithImage("busybox:latest"),
				container.WithCmd("top"),
				container.WithSysctls(tc.sysctls),
			)
			defer func() {
				c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})
			}()

			runCmd := func(ctrId string, cmd []string, expExitCode int) string {
				t.Helper()
				execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				defer cancel()
				res, err := container.Exec(execCtx, c, ctrId, cmd)
				assert.Check(t, is.Nil(err))
				assert.Check(t, is.Equal(res.ExitCode, expExitCode))
				return res.Stdout()
			}

			// Check that IPv6 is/isn't enabled, as expected.
			var expPingExitStatus int
			if !tc.expIPv6Enabled {
				expPingExitStatus = 1
			}
			runCmd(ctrId, []string{"ping", "-6", "-c1", "-W3", "::1"}, expPingExitStatus)

			// Check the contents of /etc/hosts.
			stdout := runCmd(ctrId, []string{"cat", "/etc/hosts"}, 0)
			// Append the container's own addresses/name to the expected hosts file content.
			inspect := container.Inspect(ctx, t, c, ctrId)
			exp := tc.expEtcHosts + inspect.NetworkSettings.IPAddress + "\t" + inspect.Config.Hostname + "\n"
			if tc.expIPv6Enabled {
				exp += inspect.NetworkSettings.GlobalIPv6Address + "\t" + inspect.Config.Hostname + "\n"
			}
			assert.Check(t, is.Equal(stdout, exp))
		})
	}
}

// TestEtcHostsDisconnect checks that, when a container is disconnected from a
// network, the /etc/hosts entries for that network are removed (and no others).
func TestEtcHostsDisconnect(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "/etc/hosts isn't set up on Windows")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	const netName1 = "test-etchdbr"
	network.CreateNoError(ctx, t, c, netName1,
		network.WithDriver("bridge"),
		network.WithIPv6(),
		network.WithIPAM("192.168.123.0/24", "192.168.123.1"),
		network.WithIPAM("fde6:be58:aedf::/64", "fde6:be58:aedf::1"),
	)
	defer network.RemoveNoError(ctx, t, c, netName1)

	const netName2 = "test-etchdipv"
	network.CreateNoError(ctx, t, c, netName2,
		network.WithDriver("ipvlan"),
		network.WithIPv6(),
		network.WithIPAM("192.168.124.0/24", "192.168.124.1"),
		network.WithIPAM("fdd2:c4e3:c4d5::/64", "fdd2:c4e3:c4d5::1"),
	)
	defer network.RemoveNoError(ctx, t, c, netName2)

	const ctrName = "c1"
	const ctrHostname = "c1.invalid"
	ctrId := container.Run(ctx, t, c,
		container.WithName(ctrName),
		container.WithHostname(ctrHostname),
		container.WithNetworkMode(netName1),
		container.WithExtraHost("otherhost.invalid:192.0.2.3"),
		container.WithExtraHost("otherhost.invalid:2001:db8::1234"),
	)
	defer c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})

	getEtcHosts := func() string {
		er := container.ExecT(ctx, t, c, ctrId, []string{"cat", "/etc/hosts"})
		return er.Stdout()
	}

	var err error

	// Connect a second network (don't do this in the Run, because then the /etc/hosts
	// entries for the two networks can end up in either order).
	err = c.NetworkConnect(ctx, netName2, ctrName, nil)
	assert.Check(t, err)
	golden.Assert(t, getEtcHosts(), "TestEtcHostsDisconnect1.golden")

	// Disconnect net1, its hosts entries are currently before net2's.
	err = c.NetworkDisconnect(ctx, netName1, ctrName, false)
	assert.Check(t, err)
	golden.Assert(t, getEtcHosts(), "TestEtcHostsDisconnect2.golden")

	// Reconnect net1, so that its entries will follow net2's.
	err = c.NetworkConnect(ctx, netName1, ctrName, nil)
	assert.Check(t, err)
	golden.Assert(t, getEtcHosts(), "TestEtcHostsDisconnect3.golden")

	// Disconnect net1 again, removing its entries from the end of the file.
	err = c.NetworkDisconnect(ctx, netName1, ctrName, false)
	assert.Check(t, err)
	golden.Assert(t, getEtcHosts(), "TestEtcHostsDisconnect4.golden")

	// Disconnect net2, the only network.
	err = c.NetworkDisconnect(ctx, netName2, ctrName, false)
	assert.Check(t, err)
	golden.Assert(t, getEtcHosts(), "TestEtcHostsDisconnect5.golden")
}
