//go:build linux
// +build linux

package networking

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func getIfaceAddress(t *testing.T, name string, ipv6 bool) net.IP {
	t.Helper()

	iface, err := net.InterfaceByName(name)
	assert.NilError(t, err)

	addrs, err := iface.Addrs()
	assert.NilError(t, err)
	assert.Check(t, len(addrs) > 0)

	for _, addr := range addrs {
		a := addr.(*net.IPNet)
		if !ipv6 && a.IP.To4() != nil {
			return a.IP
		}
		if ipv6 && a.IP.To4() == nil {
			return a.IP
		}
	}

	t.Fatalf("could not find an appropriate IP address attached to %s", name)
	return nil
}

type natFromLocalHostTC struct {
	name       string
	bridgeOpts []func(*types.NetworkCreate)
	clientAddr net.IP
	skipMsg    string
}

// TestAccessPublishedPortFromLocalHost checks whether published ports are accessible when a combination of the
// following options are used:
//  1. IPv4 and IPv6 ;
//  2. Loopback address, and any other local address ;
//  3. With and without userland proxy enabled ;
func TestAccessPublishedPortFromLocalHost(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless())

	testcases := []natFromLocalHostTC{
		{
			name:       "IPv4 - with loopback address",
			clientAddr: getIfaceAddress(t, "lo", false),
		},
		{
			name:       "IPv4 - with local IP address",
			clientAddr: getIfaceAddress(t, "eth0", false),
		},
		{
			name:       "IPv6 - with loopback address",
			clientAddr: getIfaceAddress(t, "lo", true),
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b237::/64", "fdf1:a844:380c:b237::1"),
			},
			skipMsg: "This test never passes",
		},
		{
			name:       "IPv6 - with local IP address",
			clientAddr: getIfaceAddress(t, "eth0", true),
			bridgeOpts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:380c:b247::/64", "fdf1:a844:380c:b247::1"),
			},
			skipMsg: "This test never passes",
		},
	}

	tester := func(t *testing.T, d *daemon.Daemon, c *client.Client, tcID int, tc natFromLocalHostTC) {
		ctx := context.Background()

		// Sending and receiving some data is an important part of the test. If we don't do that, the TCP handshake
		// might succeed by connecting to something else than the "server" container used below. This could happen for
		// instance when one of the bridge subnet is used by another interface on the host (and there might be other
		// cases). Thus, this is preventing false positives.
		msg := "hello world"
		serverPort := 1234 + tcID
		serverCmd := fmt.Sprintf("echo %q | nc -l -p %d", msg, serverPort)

		bridgeName := fmt.Sprintf("nat-lo-%d", tcID)
		network.CreateNoError(ctx, t, c, bridgeName, append(tc.bridgeOpts,
			network.WithDriver("bridge"),
			network.WithOption("com.docker.network.bridge.name", bridgeName))...)
		defer network.RemoveNoError(ctx, t, c, bridgeName)

		ctrName := sanitizeCtrName(t.Name() + "-server")
		publishSpec := fmt.Sprintf("%d:%d", serverPort, serverPort)
		ctr1 := container.Run(ctx, t, c,
			container.WithName(ctrName),
			container.WithImage("busybox:latest"),
			container.WithPublishedPorts(container.MustParsePortSpec(t, publishSpec)),
			container.WithCmd("/bin/sh", "-c", serverCmd),
			container.WithNetworkMode(bridgeName))
		defer c.ContainerRemove(ctx, ctr1, types.ContainerRemoveOptions{
			Force: true,
		})

		poll.WaitOn(t, container.IsInState(ctx, c, ctrName, "running"), poll.WithDelay(100*time.Millisecond))

		dialer := &net.Dialer{
			Timeout: 3 * time.Second,
		}
		conn, err := dialer.Dial("tcp", net.JoinHostPort(tc.clientAddr.String(), strconv.Itoa(serverPort)))
		assert.NilError(t, err)
		defer conn.Close()

		data, err := io.ReadAll(conn)
		assert.NilError(t, err)
		assert.Check(t, is.Equal(msg, strings.TrimSpace(string(data))))
	}

	for flagID, flag := range []string{"--userland-proxy=true", "--userland-proxy=false"} {
		t.Run(flag, func(t *testing.T) {
			d := daemon.New(t)
			d.StartWithBusybox(t, "--experimental", "--ip6tables", flag)
			defer d.Stop(t)

			c := d.NewClientT(t)
			defer c.Close()

			for tcID, tc := range testcases {
				// tcID is made unique across all t.Run() to make sure bridge names are unique.
				tcID = flagID*len(testcases) + tcID

				t.Run(tc.name, func(t *testing.T) {
					skip.If(t, tc.skipMsg != "", tc.skipMsg)
					tester(t, d, c, tcID, tc)
				})
			}
		})
	}
}

type accessFromBridgeGatewayTC struct {
	name        string
	ipv6        bool
	bridge1Opts []func(create *types.NetworkCreate)
	bridge2Opts []func(create *types.NetworkCreate)
	skipMsg     string
}

func TestAccessPublishedPortFromBridgeGateway(t *testing.T) {
	skip.If(t, testEnv.DaemonInfo.OSType == "windows")
	skip.If(t, testEnv.IsRootless())

	ulpTestcases := []struct {
		daemonFlag string
		skipMsg    string
	}{
		{daemonFlag: "--userland-proxy=true"},
		{daemonFlag: "--userland-proxy=false", skipMsg: "See moby/moby#38784"},
	}
	testcases := []accessFromBridgeGatewayTC{
		{
			name: "IPv4",
		},
		{
			name: "IPv6 - with unique local address",
			ipv6: true,
			bridge1Opts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:310c:b237::/64", "fdf1:a844:310c:b237::1"),
			},
			bridge2Opts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("fdf1:a844:310c:b247::/64", "fdf1:a844:310c:b247::1"),
			},
			skipMsg: "Containers with IPv6 ULAs can't reach ports published from another bridge",
		},
		{
			name: "IPv6 - with global address",
			ipv6: true,
			bridge1Opts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("2001:db8:1531::/64", "2001:db8:1531::1"),
			},
			bridge2Opts: []func(*types.NetworkCreate){
				network.WithIPv6(),
				network.WithIPAM("2001:db8:1532::/64", "2001:db8:1532::1"),
			},
		},
	}

	tester := func(t *testing.T, d *daemon.Daemon, c *client.Client, tcID int, tc accessFromBridgeGatewayTC) {
		ctx := context.Background()

		// Sending and receiving some data is an important part of the test. See the comment in
		// TestAccessPublishedPortFromLocalHost for more details.
		msg := "hello world"
		serverPort := 1234 + tcID
		serverCmd := fmt.Sprintf("echo %q | nc -l -p %d", msg, serverPort)

		bridge1Name := fmt.Sprintf("nat-remote-%d-1", tcID)
		network.CreateNoError(ctx, t, c, bridge1Name, append(tc.bridge1Opts,
			network.WithDriver("bridge"),
			network.WithOption("com.docker.network.bridge.name", bridge1Name))...)
		defer network.RemoveNoError(ctx, t, c, bridge1Name)

		ctr1Name := sanitizeCtrName(t.Name() + "-server")
		publishSpec := fmt.Sprintf("%d:%d", serverPort, serverPort)
		ctr1 := container.Run(ctx, t, c,
			container.WithName(ctr1Name),
			container.WithImage("busybox:latest"),
			container.WithPublishedPorts(container.MustParsePortSpec(t, publishSpec)),
			container.WithCmd("sh", "-c", serverCmd),
			container.WithNetworkMode(bridge1Name))
		defer c.ContainerRemove(ctx, ctr1, types.ContainerRemoveOptions{
			Force: true,
		})

		poll.WaitOn(t, container.IsInState(ctx, c, ctr1Name, "running"), poll.WithDelay(100*time.Millisecond))

		bridge2Name := fmt.Sprintf("nat-remote-%d-2", tcID)
		network.CreateNoError(ctx, t, c, bridge2Name, append(tc.bridge2Opts,
			network.WithDriver("bridge"),
			network.WithOption("com.docker.network.bridge.name", bridge2Name))...)
		defer network.RemoveNoError(ctx, t, c, bridge2Name)

		clientCmd := fmt.Sprintf(`echo "" | nc $(ip route | awk '/default/{print $3}') %d`, serverPort)
		if tc.ipv6 {
			clientCmd = fmt.Sprintf(`echo "" | nc $(ip -6 route | awk '/default/{print $3}') %d`, serverPort)
		}

		ctr2Name := sanitizeCtrName(t.Name() + "-client")
		attachCtx, cancelCtx := context.WithTimeout(ctx, 3*time.Second)
		defer cancelCtx()
		ctr2Result := container.RunAttach(attachCtx, t, c,
			container.WithName(ctr2Name),
			container.WithImage("busybox:latest"),
			container.WithCmd("/bin/sh", "-c", clientCmd),
			container.WithNetworkMode(bridge2Name))
		defer c.ContainerRemove(ctx, ctr2Result.ContainerID, types.ContainerRemoveOptions{
			Force: true,
		})

		assert.NilError(t, ctx.Err())
		assert.Equal(t, ctr2Result.ExitCode, 0)
		assert.Check(t, is.Equal(msg, strings.TrimSpace(ctr2Result.Stdout.String())))
	}

	for ulpTCID, ulpTC := range ulpTestcases {
		t.Run(ulpTC.daemonFlag, func(t *testing.T) {
			skip.If(t, ulpTC.skipMsg != "", ulpTC.skipMsg)

			d := daemon.New(t)
			d.StartWithBusybox(t, "--experimental", "--ip6tables", ulpTC.daemonFlag)
			defer d.Stop(t)

			c := d.NewClientT(t)
			defer c.Close()

			for tcID, tc := range testcases {
				// tcID is made unique across all t.Run() to make sure bridge names are unique.
				tcID = ulpTCID*len(testcases) + tcID

				t.Run(tc.name, func(t *testing.T) {
					skip.If(t, tc.skipMsg != "", tc.skipMsg)
					tester(t, d, c, tcID, tc)
				})
			}
		})
	}
}
