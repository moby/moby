package networking

import (
	"context"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
fe00::0	ip6-localnet
ff00::0	ip6-mcastprefix
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
