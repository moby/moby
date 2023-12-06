package networking

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/skip"
)

type netOptsList []func(*types.NetworkCreate)

// Check that the '/etc/hosts' file in a container is updated according to
// whether it's connected to an IPv6 network.
func TestEtcHostsUpdates(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "-D", "--experimental", "--ip6tables")
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	// Create two networks, one without IPv6, one with.

	createNetwork := func(name string, opts netOptsList) func() {
		t.Helper()
		opts = append(opts, netOptsList{
			network.WithDriver("bridge"),
			network.WithOption("com.docker.network.bridge.name", name),
		}...)
		network.CreateNoError(ctx, t, c, name, opts...)
		return func() {
			network.RemoveNoError(ctx, t, c, name)
		}
	}

	const netName4, netName6 = "testnet4", "testnet6"
	defer createNetwork(netName4, netOptsList{})()
	defer createNetwork(netName6, netOptsList{
		network.WithIPv6(),
		network.WithIPAM("fd32:56f2:1f81::/64", "fd32:56f2:1f81::1"),
	})()

	// Run a container, connected to the IPv4 network.

	const ctrName = "test_container"
	ctrId := container.Run(ctx, t, c,
		container.WithName(ctrName),
		container.WithImage("busybox:latest"),
		container.WithCmd("top"),
		container.WithNetworkMode(netName4),
	)
	defer c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{
		Force: true,
	})

	runCmd := func(cmd []string, expExitCode int, expStdout, expStderr []string) {
		t.Helper()
		execCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		res, err := container.Exec(execCtx, c, ctrId, cmd)
		assert.Check(t, is.Nil(err))
		assert.Check(t, is.Equal(res.ExitCode, expExitCode))
		if len(expStdout) == 0 {
			assert.Check(t, is.Len(res.Stdout(), 0), "Check stdout")
		} else {
			for _, s := range expStdout {
				assert.Check(t, is.Contains(res.Stdout(), s), "Check stdout")
			}
		}
		if len(expStderr) == 0 {
			assert.Check(t, is.Len(res.Stderr(), 0), "Check stderr")
		} else {
			for _, s := range expStderr {
				assert.Check(t, is.Contains(res.Stderr(), s), "Check stderr")
			}
		}
	}

	catEtcHosts := []string{"cat", "/etc/hosts"}
	pingIP6LoAddr := []string{"ping", "-6", "-c1", "-W3", "::1"}
	pingIP6Localhost := []string{"ping", "-6", "-c1", "-W3", "ip6-localhost"}
	pingIP4Localhost := []string{"ping", "-c1", "-W3", "localhost"}

	checkNoIPv6 := func() {
		t.Helper()
		runCmd(catEtcHosts, 0,
			[]string{"127.0.0.1\tlocalhost"},
			nil)
		runCmd(pingIP6LoAddr, 1,
			[]string{"PING ::1 (::1)"},
			[]string{"sendto: Cannot assign requested address"})
		if !testEnv.IsRootless() {
			// The host's '/etc/hosts' file isn't normally used to generate DNS records
			// inside a container. But, in rootless mode, it is. So, in most cases, with no
			// '/etc/hosts' entry for 'ip6-localhost' in the container it is not resolvable.
			// But, in rootless mode, the container's behaviour depends on the content of the
			// host's '/etc/hosts'.
			runCmd(pingIP6Localhost, 1, nil,
				[]string{"ping: bad address 'ip6-localhost'"})
		}
		runCmd(pingIP4Localhost, 0,
			[]string{"1 packets transmitted, 1 packets received, 0% packet loss"},
			nil)
	}

	checkHaveIPv6 := func() {
		t.Helper()
		runCmd(catEtcHosts, 0,
			[]string{"127.0.0.1\tlocalhost", "::1\tlocalhost ip6-localhost ip6-loopback"},
			nil)
		runCmd(pingIP6LoAddr, 0,
			[]string{"1 packets transmitted, 1 packets received, 0% packet loss"},
			nil)
		runCmd(pingIP6Localhost, 0,
			[]string{"1 packets transmitted, 1 packets received, 0% packet loss"},
			nil)
		runCmd(pingIP4Localhost, 0,
			[]string{"1 packets transmitted, 1 packets received, 0% packet loss"},
			nil)
	}

	// The container is only attached to an IPv4 network, it shouldn't have the
	// built-in IPv6 entries in "/etc/hosts".
	checkNoIPv6()

	// Attach the container to an IPv6 network, check that IPv6 loopback starts
	// working and /etc/hosts has been updated.
	err := c.NetworkConnect(ctx, netName6, ctrId, &networktypes.EndpointSettings{})
	assert.NilError(t, err)
	checkHaveIPv6()

	// Detach from the IPv6 network, check that IPv6 loopback stops working.
	err = c.NetworkDisconnect(ctx, netName6, ctrId, true)
	assert.NilError(t, err)
	checkNoIPv6()
}
