package network // import "github.com/docker/docker/integration/network"

import (
	"testing"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDaemonDNSFallback(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsUserNamespace)
	ctx := testutil.StartSpan(baseContext, t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-b", "none", "--dns", "127.127.127.1", "--dns", "8.8.8.8")
	defer d.Stop(t)

	c := d.NewClientT(t)

	network.CreateNoError(ctx, t, c, "test")
	defer c.NetworkRemove(ctx, "test")

	cid := container.Run(ctx, t, c, container.WithNetworkMode("test"), container.WithCmd("nslookup", "docker.com"))
	defer c.ContainerRemove(ctx, cid, containertypes.RemoveOptions{Force: true})

	poll.WaitOn(t, container.IsSuccessful(ctx, c, cid))
}

// Check that, when the internal DNS server's address is supplied as an external
// DNS server, the daemon doesn't start talking to itself.
func TestIntDNSAsExtDNS(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "cannot start daemon on Windows test run")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name        string
		extServers  []string
		expExitCode int
		expStdout   string
	}{
		{
			name:        "only self",
			extServers:  []string{"127.0.0.11"},
			expExitCode: 1,
			expStdout:   "SERVFAIL",
		},
		{
			name:        "self then ext",
			extServers:  []string{"127.0.0.11", "8.8.8.8"},
			expExitCode: 0,
			expStdout:   "Non-authoritative answer",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			const netName = "testnet"
			network.CreateNoError(ctx, t, c, netName)
			defer network.RemoveNoError(ctx, t, c, netName)

			res := container.RunAttach(ctx, t, c,
				container.WithNetworkMode(netName),
				container.WithDNS(tc.extServers),
				container.WithCmd("nslookup", "docker.com"),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})

			assert.Check(t, is.Equal(res.ExitCode, tc.expExitCode))
			assert.Check(t, is.Contains(res.Stdout.String(), tc.expStdout))
		})
	}
}

// TestExtDNSInIPv6OnlyNw checks that an IPv6-only bridge network has external
// DNS access.
func TestExtDNSInIPv6OnlyNw(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "No IPv6 Windows")
	skip.If(t, testEnv.IsRootless, "Can't use resolver on host in rootless mode")
	ctx := setupTest(t)

	// Start a DNS server on the loopback interface.
	network.StartDaftDNS(t, "127.0.0.1")

	// Set up a temp resolv.conf pointing at that DNS server, and a daemon using it.
	tmpFileName := network.WriteTempResolvConf(t, "127.0.0.1")
	d := daemon.New(t, daemon.WithEnvVars("DOCKER_TEST_RESOLV_CONF_PATH="+tmpFileName), daemon.WithExperimental())
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	netName := "v6only"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithIPv4(false),
		network.WithIPv6(),
	)
	defer network.RemoveNoError(ctx, t, c, netName)

	ctrId := container.Run(ctx, t, c, container.WithNetworkMode(netName))
	defer c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})

	res, err := container.Exec(ctx, c, ctrId, []string{"nslookup", "test.example"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Contains(res.Stdout(), network.DNSRespAddr))
}
