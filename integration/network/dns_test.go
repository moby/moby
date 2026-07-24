package network

import (
	"net/netip"
	"testing"

	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/internal/sliceutil"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func TestDaemonDNSFallback(t *testing.T) {
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")
	skip.If(t, testEnv.DaemonInfo.OSType != "linux")
	skip.If(t, testEnv.IsUserNamespace)
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-b", "none", "--dns", "127.127.127.1", "--dns", "8.8.8.8")
	defer d.Stop(t)

	c := d.NewClientT(t)

	network.CreateNoError(ctx, t, c, "test")
	defer c.NetworkRemove(ctx, "test", client.NetworkRemoveOptions{})

	cid := container.Run(ctx, t, c, container.WithNetworkMode("test"), container.WithCmd("nslookup", "docker.com"))
	defer c.ContainerRemove(ctx, cid, client.ContainerRemoveOptions{Force: true})

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
				container.WithDNS(sliceutil.Map(tc.extServers, netip.MustParseAddr)),
				container.WithCmd("nslookup", "docker.com"),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, client.ContainerRemoveOptions{Force: true})

			assert.Check(t, is.Equal(res.ExitCode, tc.expExitCode))
			assert.Check(t, is.Contains(res.Stdout.String(), tc.expStdout))
		})
	}
}

// TestCrossNetworkInternalDomain checks that a container can resolve another
// container in a separate user-defined network using the
// "<container>.<network>.internal" pseudo-domain added for
// https://github.com/moby/moby/issues/49572.
func TestCrossNetworkInternalDomain(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows", "test only works on linux")
	skip.If(t, testEnv.IsRemoteDaemon, "cannot start daemon on remote test run")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const (
		netA      = "x49572_netA"
		netB      = "x49572_netB"
		c1        = "x49572_c1"
		c2        = "x49572_c2"
		isolated  = "x49572_isolated"
		isolated2 = "x49572_isolated_target"
	)

	network.CreateNoError(ctx, t, c, netA)
	defer network.RemoveNoError(ctx, t, c, netA)
	network.CreateNoError(ctx, t, c, netB)
	defer network.RemoveNoError(ctx, t, c, netB)
	network.CreateNoError(ctx, t, c, isolated, network.WithInternal())
	defer network.RemoveNoError(ctx, t, c, isolated)

	c1Id := container.Run(ctx, t, c,
		container.WithName(c1),
		container.WithNetworkMode(netA),
	)
	defer c.ContainerRemove(ctx, c1Id, client.ContainerRemoveOptions{Force: true})

	c2Id := container.Run(ctx, t, c,
		container.WithName(c2),
		container.WithNetworkMode(netB),
	)
	defer c.ContainerRemove(ctx, c2Id, client.ContainerRemoveOptions{Force: true})

	isoId := container.Run(ctx, t, c,
		container.WithName(isolated2),
		container.WithNetworkMode(isolated),
	)
	defer c.ContainerRemove(ctx, isoId, client.ContainerRemoveOptions{Force: true})

	tests := []struct {
		name      string
		query     string
		expectOK  bool
		expectOut string
	}{
		{
			name:      "cross-network resolves",
			query:     c2 + "." + netB + ".internal",
			expectOK:  true,
			expectOut: "Address",
		},
		{
			name:      "internal network is excluded",
			query:     isolated2 + "." + isolated + ".internal",
			expectOK:  false,
			expectOut: "",
		},
		{
			name:      "unknown network",
			query:     c2 + ".does-not-exist.internal",
			expectOK:  false,
			expectOut: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := container.Exec(ctx, c, c1Id, []string{"nslookup", tc.query})
			assert.NilError(t, err)
			if tc.expectOK {
				assert.Check(t, is.Equal(res.ExitCode, 0), "stdout=%q stderr=%q", res.Stdout(), res.Stderr())
				assert.Check(t, is.Contains(res.Stdout(), tc.expectOut))
			} else {
				assert.Check(t, res.ExitCode != 0, "expected nslookup to fail, stdout=%q", res.Stdout())
			}
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
	d := daemon.New(t,
		daemon.WithResolvConf(network.GenResolvConf("127.0.0.1")),
	)
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
	defer c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})

	res, err := container.Exec(ctx, c, ctrId, []string{"nslookup", "test.example"})
	assert.NilError(t, err)
	assert.Check(t, is.Equal(res.ExitCode, 0))
	assert.Check(t, is.Contains(res.Stdout(), network.DNSRespAddr))
}
