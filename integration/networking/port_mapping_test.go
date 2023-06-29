//go:build linux
// +build linux

package networking

import (
	"context"
	"errors"
	"fmt"
	"hash/adler32"
	"io"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
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

// synProbeFromAnotherHost sends a syn probe to destIP:destPort from an attacker simulated as being another host on a
// L2 segment shared with the victim host, which is the current network namespace. It returns an error if the victim
// host returns back a SYN-ACK. The sgmtNw parameter is the subnet used by the victim, the attacker and the bridge.
//
// If the env var TEST_MANUAL_DEBUG is specified and the test fails, the simulated L2 segment won't be destroyed. If
// TEST_REUSE_L2SEGMENT is specified, it tries to reuse an existing L2 segment from a prior run, if there's one. This
// allows to easily run tcpdump and other debugging tools while re-running a test.
func synProbeFromAnotherHost(t *testing.T, sgmtNw netip.Prefix, destIP netip.Addr, destPort uint16) error {
	manualDebug := os.Getenv("TEST_MANUAL_DEBUG") != ""
	reusePrevious := os.Getenv("TEST_REUSE_L2SEGMENT") != ""

	// The Adler-32 checksum is computed from the test name to make sure the netns created for a given test is unique
	// across tests, and stable over time. This is intended to support TEST_REUSE_L2SEGMENT.
	testID := adler32.Checksum([]byte(t.Name()))

	sgmt, err := networking.NewL2Segment(t, "br-l2-segment", sgmtNw, testID, reusePrevious)
	if err != nil {
		return fmt.Errorf("failed to create L2Segment: %w", err)
	}

	// The current netns is used since this is where the container port is published.
	victimNs := networking.CurrentNetns(t)
	victimHost := networking.L3Host{
		Ns:               victimNs,
		HostIfaceName:    fmt.Sprintf("victim-%8x", testID),
		BridgedIfaceName: "victim",
	}
	if err := sgmt.AddHost(&victimHost, reusePrevious); err != nil {
		return fmt.Errorf("failed to create victim host: %w", err)
	}

	attackerNs := networking.NewNamedNetns(t, fmt.Sprintf("attacker-%8x", testID), reusePrevious)
	attackerHost := networking.L3Host{
		Ns:               attackerNs,
		HostIfaceName:    "eth0",
		BridgedIfaceName: "attacker",
	}
	if err := sgmt.AddHost(&attackerHost, reusePrevious); err != nil {
		return fmt.Errorf("failed to create attacker host: %w", err)
	}

	t.Cleanup(func() {
		if !manualDebug {
			// A veth pair can be deleted in two ways: 1. by removing one of its interface; 2. by deleting the netns
			// where one of the interface leaves. L2Segment.Destroy() destroys the bridge netns, so this is enough to
			// tear down the whole attacker's netns and veth pair, and victim's veth pair.
			attackerNs.Destroy(t)
			sgmt.Destroy(t)
			return
		}

		fmt.Println("L2 segment is kept for manual debugging:")
		fmt.Printf("\t* Bridge netns: %s\n", sgmt.BridgeNs)
		fmt.Printf("\t* Attacker netns: %s\n", attackerNs)
		fmt.Printf("\t* Victim MAC: %s\n", victimHost.MACAddr)
		fmt.Printf("\t* Attacker MAC: %s\n", attackerHost.MACAddr)
		fmt.Printf("\t* Victim IP address: %s\n", victimHost.IPAddr)
		fmt.Printf("\t* Attacker IP address: %s\n", attackerHost.IPAddr)
	})

	prober := networking.SynProber{
		Iface:   attackerHost.HostIfaceName,
		SrcMAC:  attackerHost.MACAddr,
		DstMAC:  victimHost.MACAddr,
		SrcIP:   attackerHost.IPAddr,
		DstIP:   destIP,
		SrcPort: 60000,
		DstPort: destPort,
	}

	// We need to warm-up victim's arp table, otherwise victim host will send an ARP request like:
	// "Who has 192.168.210.3? Tell 127.0.0.1". This is obviously going to fail.
	if err := attackerNs.InNetns(t, func() error {
		t.Logf("Warm-up victim's ARP table by trying to connect to %s:%d", victimHost.IPAddr, prober.DstPort)

		// This warm-up connection is specifically using the DstPort we want to test to make sure it's not reachable.
		// This helps prevent false negatives.
		conn, err := net.Dial("tcp4", fmt.Sprintf("%s:%d", victimHost.IPAddr, prober.DstPort))
		if err != nil && !errors.Is(err, syscall.ECONNREFUSED) {
			return fmt.Errorf("warm up connection to %s:%d failed with an unexpected error: %w", victimHost.IPAddr, prober.DstPort, err)
		}
		if conn != nil {
			conn.Close()
			return fmt.Errorf("connection to %s:%d should fail, but did not. Test can't be conducted", victimHost.IPAddr, prober.DstPort)
		}
		t.Logf("Warm-up connection was correctly refused.")

		return nil
	}); err != nil {
		return fmt.Errorf("failed to warm-up victim's arp table: %w", err)
	}

	if err := attackerNs.InNetns(t, func() error {
		err = prober.Probe(deadline)
		if errors.Is(err, networking.ErrNoSynAck) {
			if manualDebug {
				manualDebug = false
				fmt.Println("Test env was previously marked persistent for manual debugging but the test succeeded; let's delete it.")
			}
			return nil
		} else if err == nil {
			err = errors.New("a SYN-ACK packet was received although we expect to not receive one")
		}

		return err
	}); err != nil {
		return fmt.Errorf("failed to conduct attack: %w", err)
	}

	return nil
}

func TestAccessPortPublishedToLoopbackFromAnotherHost(t *testing.T) {
	t.Skip("See moby/moby#45610")

	d := daemon.New(t)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	ports, portBindings, err := nat.ParsePortSpecs([]string{"127.0.0.1:1324:1324"})
	assert.NilError(t, err)

	ctx := context.Background()
	cid := container.Run(ctx, t, c,
		container.WithImage("busybox:latest"),
		container.WithPublishedPorts(ports, portBindings),
		container.WithCmd("nc", "-l", "-p", "1324"))
	defer c.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{
		Force: true,
	})

	assert.NilError(t, synProbeFromAnotherHost(t, netip.MustParsePrefix("192.168.210.1/24"),
		netip.MustParseAddr("127.0.0.1"), 1324))
}

func TestAccessUnpublishedPortFromAnotherHost(t *testing.T) {
	t.Skip("See moby/moby#45610")

	d := daemon.New(t)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	ctx := context.Background()
	cid := container.Run(ctx, t, c,
		container.WithImage("busybox:latest"),
		container.WithCmd("nc", "-l", "-p", "1324"))
	defer c.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{
		Force: true,
	})

	inspect := container.Inspect(ctx, t, c, cid)
	destIP := netip.MustParseAddr(inspect.NetworkSettings.Networks["bridge"].IPAddress)

	assert.NilError(t, synProbeFromAnotherHost(t, netip.MustParsePrefix("192.168.212.1/24"), destIP, 1324))
}

func TestAccessPortPublishedToASpecificIPFromAnotherHost(t *testing.T) {
	t.Skip("See moby/moby#45610")

	d := daemon.New(t)
	d.StartWithBusybox(t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	bindIP := getIfaceAddress(t, "eth0", false)
	ports, portBindings, err := nat.ParsePortSpecs([]string{bindIP.String() + ":1312:1312"})
	assert.NilError(t, err)

	ctx := context.Background()
	cid := container.Run(ctx, t, c,
		container.WithImage("busybox:latest"),
		container.WithPublishedPorts(ports, portBindings),
		container.WithCmd("nc", "-l", "-p", "1312"))
	defer c.ContainerRemove(ctx, cid, types.ContainerRemoveOptions{
		Force: true,
	})

	destIP, _ := netip.AddrFromSlice(bindIP)
	assert.NilError(t, synProbeFromAnotherHost(t, netip.MustParsePrefix("192.168.213.1/24"), destIP, 1312))
}
