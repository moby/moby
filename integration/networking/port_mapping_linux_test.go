package networking

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

func TestAccessPublishedPortFromCtr(t *testing.T) {
	// This test makes changes to the host's "/proc/sys/net/bridge/bridge-nf-call-iptables",
	// which would have no effect on rootlesskit's netns.
	skip.If(t, testEnv.IsRootless, "rootlesskit has its own netns")

	testcases := []struct {
		name            string
		daemonOpts      []string
		disableBrNfCall bool
	}{
		{
			name: "with-proxy",
		},
		{
			name:       "no-proxy",
			daemonOpts: []string{"--userland-proxy=false"},
		},
		{
			// Before starting the daemon, disable bridge-nf-call-iptables. It should
			// be enabled by the daemon because, without docker-proxy, it's needed to
			// DNAT packets crossing the bridge between containers.
			// Regression test for https://github.com/moby/moby/issues/48664
			name:            "no-proxy no-brNfCall",
			daemonOpts:      []string{"--userland-proxy=false"},
			disableBrNfCall: true,
		},
	}

	// Find an address on the test host.
	hostAddr := func() string {
		conn, err := net.Dial("tcp4", "hub.docker.com:80")
		assert.NilError(t, err)
		defer conn.Close()
		return conn.LocalAddr().(*net.TCPAddr).IP.String()
	}()

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := setupTest(t)

			if tc.disableBrNfCall {
				// Only run this test if br_netfilter is loaded, and enabled for IPv4.
				const procFile = "/proc/sys/net/bridge/bridge-nf-call-iptables"
				val, err := os.ReadFile(procFile)
				if err != nil {
					t.Skipf("Cannot read %s, br_netfilter not loaded? (%s)", procFile, err)
				}
				if val[0] != '1' {
					t.Skipf("bridge-nf-call-iptables=%v", val[0])
				}
				err = os.WriteFile(procFile, []byte{'0', '\n'}, 0o644)
				assert.NilError(t, err)
				defer os.WriteFile(procFile, []byte{'1', '\n'}, 0o644)
			}

			d := daemon.New(t)
			d.StartWithBusybox(ctx, t, tc.daemonOpts...)
			defer d.Stop(t)
			c := d.NewClientT(t)
			defer c.Close()

			const netName = "tappfcnet"
			network.CreateNoError(ctx, t, c, netName)
			defer network.RemoveNoError(ctx, t, c, netName)

			serverId := container.Run(ctx, t, c,
				container.WithNetworkMode(netName),
				container.WithExposedPorts("80"),
				container.WithPortMap(nat.PortMap{"80": {{HostIP: "0.0.0.0"}}}),
				container.WithCmd("httpd", "-f"),
			)
			defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

			inspect := container.Inspect(ctx, t, c, serverId)
			hostPort := inspect.NetworkSettings.Ports["80/tcp"][0].HostPort

			clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(clientCtx, t, c,
				container.WithNetworkMode(netName),
				container.WithCmd("wget", "http://"+net.JoinHostPort(hostAddr, hostPort)),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})
			assert.Check(t, is.Contains(res.Stderr.String(), "404 Not Found"))
		})
	}
}
