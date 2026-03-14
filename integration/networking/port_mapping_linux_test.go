package networking

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/moby/moby/api/pkg/stdcopy"
	networktypes "github.com/moby/moby/api/types/network"
	"github.com/moby/moby/client"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/integration/internal/container"
	"github.com/moby/moby/v2/integration/internal/network"
	"github.com/moby/moby/v2/integration/internal/testutils/networking"
	"github.com/moby/moby/v2/internal/testutil"
	"github.com/moby/moby/v2/internal/testutil/daemon"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

func getIfaceAddrs(t *testing.T, name string, ipv6 bool) []net.IP {
	t.Helper()

	iface, err := net.InterfaceByName(name)
	assert.NilError(t, err)

	addrs, err := iface.Addrs()
	assert.NilError(t, err)

	var ips []net.IP

	for _, netaddr := range addrs {
		addr := netaddr.(*net.IPNet)
		if (addr.IP.To4() != nil && !ipv6) || (addr.IP.To4() == nil && ipv6) {
			ips = append(ips, addr.IP)
		}
	}

	assert.Check(t, len(ips) > 0)
	return ips
}

func TestDisableNAT(t *testing.T) {
	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	testcases := []struct {
		name       string
		gwMode4    string
		gwMode6    string
		expPortMap networktypes.PortMap
	}{
		{
			name: "defaults",
			expPortMap: networktypes.PortMap{
				networktypes.MustParsePort("80/tcp"): []networktypes.PortBinding{
					{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "8080"},
					{HostIP: netip.MustParseAddr("::"), HostPort: "8080"},
				},
			},
		},
		{
			name:    "nat4 routed6",
			gwMode4: "nat",
			gwMode6: "routed",
			expPortMap: networktypes.PortMap{
				networktypes.MustParsePort("80/tcp"): []networktypes.PortBinding{
					{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: "8080"},
					{HostIP: netip.MustParseAddr("::"), HostPort: ""},
				},
			},
		},
		{
			name:    "nat6 routed4",
			gwMode4: "routed",
			gwMode6: "nat",
			expPortMap: networktypes.PortMap{
				networktypes.MustParsePort("80/tcp"): []networktypes.PortBinding{
					{HostIP: netip.MustParseAddr("::"), HostPort: "8080"},
					{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: ""},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			const netName = "testnet"
			nwOpts := []func(options *client.NetworkCreateOptions){
				network.WithIPv6(),
				network.WithIPAM("fd2a:a2c3:4448::/64", "fd2a:a2c3:4448::1"),
			}
			if tc.gwMode4 != "" {
				nwOpts = append(nwOpts, network.WithOption(bridge.IPv4GatewayMode, tc.gwMode4))
			}
			if tc.gwMode6 != "" {
				nwOpts = append(nwOpts, network.WithOption(bridge.IPv6GatewayMode, tc.gwMode6))
			}
			network.CreateNoError(ctx, t, c, netName, nwOpts...)
			defer network.RemoveNoError(ctx, t, c, netName)

			id := container.Run(ctx, t, c,
				container.WithNetworkMode(netName),
				container.WithExposedPorts("80/tcp"),
				container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80/tcp"): {{HostPort: "8080"}}}),
			)
			defer c.ContainerRemove(ctx, id, client.ContainerRemoveOptions{Force: true})

			inspect := container.Inspect(ctx, t, c, id)
			assert.Check(t, is.DeepEqual(inspect.NetworkSettings.Ports, tc.expPortMap, cmpopts.EquateComparable(netip.Addr{})))
		})
	}
}

// Check that a container on one network can reach a TCP service in a container
// on another network, via a mapped port on the host.
func TestPortMappedHairpinTCP(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Find an address on the test host.
	conn, err := net.Dial("tcp4", "hub.docker.com:80")
	assert.NilError(t, err)
	hostAddr := conn.LocalAddr().(*net.TCPAddr).IP.String()
	conn.Close()

	const serverNetName = "servernet"
	network.CreateNoError(ctx, t, c, serverNetName)
	defer network.RemoveNoError(ctx, t, c, serverNetName)
	const clientNetName = "clientnet"
	network.CreateNoError(ctx, t, c, clientNetName)
	defer network.RemoveNoError(ctx, t, c, clientNetName)

	serverId := container.Run(ctx, t, c,
		container.WithNetworkMode(serverNetName),
		container.WithExposedPorts("80"),
		container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80"): {{HostIP: netip.MustParseAddr("0.0.0.0")}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, client.ContainerRemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports[networktypes.MustParsePort("80/tcp")][0].HostPort

	clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := container.RunAttach(clientCtx, t, c,
		container.WithNetworkMode(clientNetName),
		container.WithCmd("wget", "http://"+hostAddr+":"+hostPort),
	)
	defer c.ContainerRemove(ctx, res.ContainerID, client.ContainerRemoveOptions{Force: true})
	assert.Check(t, is.Contains(res.Stderr.String(), "404 Not Found"))
}

// Check that a container on one network can reach a UDP service in a container
// on another network, via a mapped port on the host.
// Regression test for https://github.com/moby/libnetwork/issues/1729.
func TestPortMappedHairpinUDP(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Find an address on the test host.
	conn, err := net.Dial("tcp4", "hub.docker.com:80")
	assert.NilError(t, err)
	hostAddr := conn.LocalAddr().(*net.TCPAddr).IP.String()
	conn.Close()

	const serverNetName = "servernet"
	network.CreateNoError(ctx, t, c, serverNetName)
	defer network.RemoveNoError(ctx, t, c, serverNetName)
	const clientNetName = "clientnet"
	network.CreateNoError(ctx, t, c, clientNetName)
	defer network.RemoveNoError(ctx, t, c, clientNetName)

	serverId := container.Run(ctx, t, c,
		container.WithNetworkMode(serverNetName),
		container.WithExposedPorts("54/udp"),
		container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("54/udp"): {{HostIP: netip.MustParseAddr("0.0.0.0")}}}),
		container.WithCmd("/bin/sh", "-c", "echo 'foobar.internal 192.168.155.23' | dnsd -c - -p 54"),
	)
	defer c.ContainerRemove(ctx, serverId, client.ContainerRemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports[networktypes.MustParsePort("54/udp")][0].HostPort

	// nslookup gets an answer quickly from the dns server, but then tries to
	// query another DNS server (for some unknown reasons) and times out. Hence,
	// we need >5s to execute this test.
	clientCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res := container.RunAttach(clientCtx, t, c,
		container.WithNetworkMode(clientNetName),
		container.WithCmd("nslookup", "foobar.internal", net.JoinHostPort(hostAddr, hostPort)),
		container.WithAutoRemove,
	)
	assert.Check(t, is.Contains(res.Stdout.String(), "192.168.155.23"))
}

// Check that a container on an IPv4-only network can have a port mapping
// from a specific IPv6 host address (using docker-proxy).
// Regression test for https://github.com/moby/moby/issues/48067 (which
// is about incorrectly reporting this as invalid config).
func TestProxy4To6(t *testing.T) {
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "ipv4net"
	network.CreateNoError(ctx, t, c, netName)

	serverId := container.Run(ctx, t, c,
		container.WithNetworkMode(netName),
		container.WithExposedPorts("80"),
		container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80"): {{HostIP: netip.MustParseAddr("::1")}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, client.ContainerRemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports[networktypes.MustParsePort("80/tcp")][0].HostPort

	var resp *http.Response
	addr := "http://[::1]:" + hostPort
	poll.WaitOn(t, func(t poll.LogT) poll.Result {
		var err error
		resp, err = http.Get(addr) // #nosec G107 -- Ignore "Potential HTTP request made with variable url"
		if err != nil {
			return poll.Continue("waiting for %s to be accessible: %v", addr, err)
		}
		return poll.Success()
	})
	assert.Check(t, is.Equal(resp.StatusCode, 404))
}

func enableIPv6OnAll(t *testing.T) func() {
	t.Helper()

	out, err := exec.Command("sysctl", "net.ipv6.conf").Output()
	assert.NilError(t, err)

	ifaces := map[string]string{}
	var allVal string

	sysctls := strings.SplitSeq(string(out), "\n")
	for sysctl := range sysctls {
		if sysctl == "" {
			continue
		}

		kv := strings.Split(sysctl, " = ")
		sub := strings.Split(kv[0], ".")
		if sub[4] == "disable_ipv6" {
			if sub[3] == "all" {
				allVal = kv[1]
				continue
			}
			ifaces[sub[3]] = kv[1]
		}
	}

	assert.NilError(t, exec.Command("sysctl", "net.ipv6.conf.all.disable_ipv6=0").Run())

	return func() {
		if allVal == "1" {
			assert.NilError(t, exec.Command("sysctl", "net.ipv6.conf.all.disable_ipv6=1").Run())
		}

		for iface, val := range ifaces {
			assert.NilError(t, exec.Command("sysctl", fmt.Sprintf("net.ipv6.conf.%s.disable_ipv6=%s", iface, val)).Run())
		}
	}
}

// TestAccessPublishedPortFromHost checks whether published ports are
// accessible from the host.
func TestAccessPublishedPortFromHost(t *testing.T) {
	// Both IPv6 test cases are currently failing in rootless mode. This needs further investigation.
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)

	revertIPv6OnAll := enableIPv6OnAll(t)
	defer revertIPv6OnAll()
	assert.NilError(t, exec.Command("ip", "addr", "add", "fdfb:5cbb:29bf::2/64", "dev", "eth0", "nodad").Run())
	defer func() {
		assert.NilError(t, exec.Command("ip", "addr", "del", "fdfb:5cbb:29bf::2/64", "dev", "eth0").Run())
	}()

	testcases := []struct {
		ulpEnabled bool
		ipv6       bool
	}{
		{
			ulpEnabled: true,
			ipv6:       false,
		},
		{
			ulpEnabled: false,
			ipv6:       false,
		},
		{
			ulpEnabled: true,
			ipv6:       true,
		},
		{
			ulpEnabled: false,
			ipv6:       true,
		},
	}

	for tcID, tc := range testcases {
		t.Run(fmt.Sprintf("userland-proxy=%t/IPv6=%t", tc.ulpEnabled, tc.ipv6), func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			d := daemon.New(t)
			d.StartWithBusybox(ctx, t, fmt.Sprintf("--userland-proxy=%t", tc.ulpEnabled))
			defer d.Stop(t)

			c := d.NewClientT(t)
			defer c.Close()

			bridgeName := fmt.Sprintf("nat-from-host-%d", tcID)
			bridgeOpts := []func(options *client.NetworkCreateOptions){
				network.WithDriver("bridge"),
				network.WithOption(bridge.BridgeName, bridgeName),
			}
			if tc.ipv6 {
				bridgeOpts = append(bridgeOpts,
					network.WithIPv6(),
					network.WithIPAM("fd31:1c42:6f59::/64", "fd31:1c42:6f59::1"))
			}

			network.CreateNoError(ctx, t, c, bridgeName, bridgeOpts...)
			defer network.RemoveNoError(ctx, t, c, bridgeName)

			hostPort := strconv.Itoa(1234 + tcID)
			serverID := container.Run(ctx, t, c,
				container.WithName(sanitizeCtrName(t.Name()+"-server")),
				container.WithExposedPorts("80/tcp"),
				container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80/tcp"): {{HostPort: hostPort}}}),
				container.WithCmd("httpd", "-f"),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, serverID, client.ContainerRemoveOptions{Force: true})

			for _, iface := range []string{"lo", "eth0"} {
				for _, hostAddr := range getIfaceAddrs(t, iface, tc.ipv6) {
					if !tc.ulpEnabled && hostAddr.To4() == nil && hostAddr.IsLoopback() {
						// iptables can't DNAT packets addressed to the IPv6
						// loopback address.
						continue
					}

					addr := hostAddr.String()
					if hostAddr.IsLinkLocalUnicast() {
						if !tc.ulpEnabled {
							// iptables can DNAT packets addressed to link-local
							// addresses, but they won't be SNATed, so the
							// target server won't know where to reply. Thus,
							// the userland-proxy is required for these addresses.
							continue
						}
						if networking.FirewalldRunning() {
							// FIXME(robmry) - With firewalld running, this test is flaky.
							// - it always seems to fail in CI, but not in a local dev container.
							// - tracked by https://github.com/moby/moby/issues/49695
							continue
						}
						addr += "%25" + iface
					}

					httpClient := &http.Client{Timeout: 3 * time.Second}
					resp, err := httpClient.Get("http://" + net.JoinHostPort(addr, hostPort))
					assert.NilError(t, err)
					assert.Check(t, is.Equal(resp.StatusCode, 404))
				}
			}
		})
	}
}

func TestAccessPublishedPortFromRemoteHost(t *testing.T) {
	// IPv6 test case is currently failing in rootless mode. This needs further investigation.
	skip.If(t, testEnv.IsRootless)

	ctx := setupTest(t)

	l3 := networking.NewL3Segment(t, "test-pbs-remote-br",
		netip.MustParsePrefix("192.168.120.1/24"),
		netip.MustParsePrefix("fd30:e631:f886::1/64"))
	defer l3.Destroy(t)

	// "docker" is the host where dockerd is running and where ports will be
	// published.
	l3.AddHost(t, "docker", networking.CurrentNetns, "eth-test",
		netip.MustParsePrefix("192.168.120.2/24"),
		netip.MustParsePrefix("fd30:e631:f886::2/64"))
	l3.AddHost(t, "neigh", "test-pbs-remote-neighbor", "eth0",
		netip.MustParsePrefix("192.168.120.3/24"),
		netip.MustParsePrefix("fd30:e631:f886::3/64"))

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	bridgeName := "nat-remote"
	network.CreateNoError(ctx, t, c, bridgeName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, bridgeName),
		network.WithIPv6(),
		network.WithIPAM("fdd8:c9fe:1a25::/64", "fdd8:c9fe:1a25::1"))
	defer network.RemoveNoError(ctx, t, c, bridgeName)

	hostPort := "1780"
	serverID := container.Run(ctx, t, c,
		container.WithName(sanitizeCtrName(t.Name()+"-server")),
		container.WithExposedPorts("80/tcp"),
		container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80/tcp"): {{HostPort: hostPort}}}),
		container.WithCmd("httpd", "-f"),
		container.WithNetworkMode(bridgeName))
	defer c.ContainerRemove(ctx, serverID, client.ContainerRemoveOptions{Force: true})

	for _, ipv6 := range []bool{true, false} {
		for _, hostAddr := range getIfaceAddrs(t, l3.Hosts["docker"].Iface, ipv6) {
			if hostAddr.IsLinkLocalUnicast() {
				// For some reason, hosts in a L3Segment can't communicate
				// using link-local addresses.
				continue
			}

			l3.Hosts["neigh"].Do(t, func() {
				url := "http://" + net.JoinHostPort(hostAddr.String(), hostPort)
				t.Logf("Sending a request to %s", url)

				icmd.RunCommand("curl", url).Assert(t, icmd.Success)
			})
		}
	}
}

// TestAccessPublishedPortFromCtr checks that a container's published ports can
// be reached from the container that published the ports, and a neighbouring
// container on the same network. It runs in three modes:
//
// - userland proxy enabled (default behaviour).
// - proxy disabled (https://github.com/moby/moby/issues/12632)
// - proxy disabled, 'bridge-nf-call-iptables=0' (https://github.com/moby/moby/issues/48664)
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
				container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80/tcp"): {{HostIP: netip.MustParseAddr("0.0.0.0")}}}),
				container.WithCmd("httpd", "-f"),
			)
			defer c.ContainerRemove(ctx, serverId, client.ContainerRemoveOptions{Force: true})

			inspect := container.Inspect(ctx, t, c, serverId)
			hostPort := inspect.NetworkSettings.Ports[networktypes.MustParsePort("80/tcp")][0].HostPort

			clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			res := container.RunAttach(clientCtx, t, c,
				container.WithNetworkMode(netName),
				container.WithCmd("wget", "http://"+net.JoinHostPort(hostAddr, hostPort)),
			)
			defer c.ContainerRemove(ctx, res.ContainerID, client.ContainerRemoveOptions{Force: true})
			assert.Check(t, is.Contains(res.Stderr.String(), "404 Not Found"))

			// Also check that the container can reach its own published port.
			clientCtx2, cancel2 := context.WithTimeout(ctx, 5*time.Second)
			defer cancel2()
			execRes := container.ExecT(clientCtx2, t, c, serverId, []string{"wget", "http://" + net.JoinHostPort(hostAddr, hostPort)})
			assert.Check(t, is.Contains(execRes.Stderr(), "404 Not Found"))
		})
	}
}

// TestRestartUserlandProxyUnder2MSL checks that a container can be restarted
// while previous connections to the proxy are still in TIME_WAIT state.
func TestRestartUserlandProxyUnder2MSL(t *testing.T) {
	skip.If(t, testEnv.IsRootless())

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const netName = "nat-time-wait"
	network.CreateNoError(ctx, t, c, netName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, netName))
	defer network.RemoveNoError(ctx, t, c, netName)

	ctrName := sanitizeCtrName(t.Name() + "-server")
	ctrOpts := []func(*container.TestContainerConfig){
		container.WithName(ctrName),
		container.WithExposedPorts("80/tcp"),
		container.WithPortMap(networktypes.PortMap{
			networktypes.MustParsePort("80/tcp"): {{HostPort: "1780"}},
		}),
		container.WithCmd("httpd", "-f"),
		container.WithNetworkMode(netName),
	}

	container.Run(ctx, t, c, ctrOpts...)
	defer c.ContainerRemove(ctx, ctrName, client.ContainerRemoveOptions{Force: true})

	// Make an HTTP request to open a TCP connection to the proxy. We don't
	// care about the HTTP response, just that the connection is established.
	// So, check that we receive a 404 to make sure we've a working full-duplex
	// TCP connection.
	httpClient := &http.Client{Timeout: 3 * time.Second}
	resp, err := httpClient.Get("http://127.0.0.1:1780")
	assert.NilError(t, err)
	assert.Check(t, is.Equal(resp.StatusCode, 404))

	// Removing the container will kill the userland proxy, and the connection
	// opened by the previous HTTP request will be properly closed (ie. on both
	// sides). Thus, that connection will transition to the TIME_WAIT state.
	_, err = c.ContainerRemove(ctx, ctrName, client.ContainerRemoveOptions{Force: true})
	assert.NilError(t, err)

	// Make sure the container can be restarted. [container.Run] checks that
	// the ContainerStart API call doesn't return an error. We don't need to
	// make another TCP connection either, that's out of scope. Hence, we don't
	// need to check anything after this call.
	container.Run(ctx, t, c, ctrOpts...)
}

// Test direct routing from remote hosts (setting up a route to a container
// network on a remote host, and addressing containers directly), for
// combinations of:
// - Filter FORWARD default policy: ACCEPT/DROP - shouldn't affect behaviour
// - Gateway mode: nat/routed
// For each combination, test:
// - ping
// - http access to an open (mapped) container port
// - http access to an unmapped container port
func TestDirectRoutingOpenPorts(t *testing.T) {
	skip.If(t, testEnv.IsRootless())
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	t.Cleanup(func() { d.Stop(t) })

	c := d.NewClientT(t)
	t.Cleanup(func() { c.Close() })

	// Simulate the remote host.

	l3 := networking.NewL3Segment(t, "test-routed-open-ports",
		netip.MustParsePrefix("192.168.124.1/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::1/64"))
	t.Cleanup(func() { l3.Destroy(t) })

	// "docker" is the host where dockerd is running.
	l3.AddHost(t, "docker", networking.CurrentNetns, "eth-test",
		netip.MustParsePrefix("192.168.124.2/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::2/64"))
	// "remote" simulates the remote host.
	l3.AddHost(t, "remote", "test-remote-host", "eth0",
		netip.MustParsePrefix("192.168.124.3/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::3/64"))
	// Add default routes to the "docker" Host from the "remote" Host.
	l3.Hosts["remote"].MustRun(t, "ip", "route", "add", "default", "via", "192.168.124.2")
	l3.Hosts["remote"].MustRun(t, "ip", "-6", "route", "add", "default", "via", "fdc0:36dc:a4dd::2")

	type ctrDesc struct {
		id   string
		ipv4 string
		ipv6 string
	}

	// Create a network and run a container on it.
	// Run http servers on ports 80 and 81, but only map/open port 80.
	createNet := func(gwMode string) ctrDesc {
		netName := "test-" + gwMode
		brName := "br-" + gwMode
		if len(brName) > syscall.IFNAMSIZ {
			brName = brName[:syscall.IFNAMSIZ-1]
		}
		network.CreateNoError(ctx, t, c, netName,
			network.WithDriver("bridge"),
			network.WithIPv6(),
			network.WithOption(bridge.BridgeName, brName),
			network.WithOption(bridge.IPv4GatewayMode, gwMode),
			network.WithOption(bridge.IPv6GatewayMode, gwMode),
		)
		t.Cleanup(func() {
			network.RemoveNoError(ctx, t, c, netName)
		})

		ctrId := container.Run(ctx, t, c,
			container.WithNetworkMode(netName),
			container.WithName("ctr-"+gwMode),
			container.WithExposedPorts("80/tcp"),
			container.WithPortMap(networktypes.PortMap{
				// TODO(robmry): this test supplies an empty list of PortBindings.
				// https://github.com/moby/moby/issues/51727 will break it.
				networktypes.MustParsePort("80/tcp"): {{}},
			}),
		)
		t.Cleanup(func() {
			c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})
		})

		container.ExecT(ctx, t, c, ctrId, []string{"httpd", "-p", "80"})
		container.ExecT(ctx, t, c, ctrId, []string{"httpd", "-p", "81"})

		insp := container.Inspect(ctx, t, c, ctrId)
		return ctrDesc{
			id:   ctrId,
			ipv4: insp.NetworkSettings.Networks[netName].IPAddress.String(),
			ipv6: insp.NetworkSettings.Networks[netName].GlobalIPv6Address.String(),
		}
	}

	const (
		httpSuccess = "404 Not Found"
		httpFail    = "Connection timed out"
		pingSuccess = 0
		pingFail    = 1
	)

	networks := map[string]ctrDesc{
		"nat":             createNet("nat"),
		"nat-unprotected": createNet("nat-unprotected"),
		"routed":          createNet("routed"),
	}
	expPingExit := map[string]int{
		"nat":             pingFail,
		"nat-unprotected": pingSuccess,
		"routed":          pingSuccess,
	}
	expMappedPortHTTP := map[string]string{
		"nat":             httpFail,
		"nat-unprotected": httpSuccess,
		"routed":          httpSuccess,
	}
	expUnmappedPortHTTP := map[string]string{
		"nat":             httpFail,
		"nat-unprotected": httpSuccess,
		"routed":          httpFail,
	}

	testPing := func(t *testing.T, cmd, addr string, expExit int) {
		t.Helper()
		t.Parallel()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			pingRes := icmd.RunCommand(cmd, "-n", "-c1", "-W3", addr)
			assert.Check(t, pingRes.ExitCode == expExit, "%s %s -> out:%s err:%s",
				cmd, addr, pingRes.Stdout(), pingRes.Stderr())
		})
	}
	testHttp := func(t *testing.T, addr, port, expOut string) {
		t.Helper()
		t.Parallel()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			u := "http://" + net.JoinHostPort(addr, port)
			res := icmd.RunCommand("curl", "--max-time", "3", "--show-error", "--silent", u)
			assert.Check(t, is.Contains(res.Combined(), expOut), "url:%s", u)
		})
	}

	// Run the ping and http tests in two parallel groups, rather than waiting for
	// ping/http timeouts separately. (The iptables filter-FORWARD policy affects the
	// whole host, so ACCEPT/DROP tests can't be parallelized).
	runTests := func(testName, policy string) {
		t.Run(testName, func(t *testing.T) {
			if policy != "" {
				networking.SetFilterForwardPolicies(t, policy)
			}
			for gwMode := range networks {
				t.Run(gwMode+"/v4/ping", func(t *testing.T) {
					testPing(t, "ping", networks[gwMode].ipv4, expPingExit[gwMode])
				})
				t.Run(gwMode+"/v6/ping", func(t *testing.T) {
					testPing(t, "ping6", networks[gwMode].ipv6, expPingExit[gwMode])
				})
				t.Run(gwMode+"/v4/http/80", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv4, "80", expMappedPortHTTP[gwMode])
				})
				t.Run(gwMode+"/v4/http/81", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv4, "81", expUnmappedPortHTTP[gwMode])
				})
				t.Run(gwMode+"/v6/http/80", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv6, "80", expMappedPortHTTP[gwMode])
				})
				t.Run(gwMode+"/v6/http/81", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv6, "81", expUnmappedPortHTTP[gwMode])
				})
			}
		})
	}

	if strings.HasPrefix(d.FirewallBackendDriver(t), "iptables") {
		runTests("iptables-ACCEPT", "ACCEPT")
		runTests("iptables-DROP", "DROP")
	} else {
		runTests("nftables", "")
	}
}

func TestAcceptFwMark(t *testing.T) {
	skip.If(t, testEnv.IsRootless())
	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--bridge-accept-fwmark=2/3")
	t.Cleanup(func() { d.Stop(t) })

	c := d.NewClientT(t)
	t.Cleanup(func() { c.Close() })

	// Simulate the remote host.

	l3 := networking.NewL3Segment(t, "test-routed-open-ports",
		netip.MustParsePrefix("192.168.124.1/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::1/64"))
	t.Cleanup(func() { l3.Destroy(t) })

	// "docker" is the host where dockerd is running.
	l3.AddHost(t, "docker", networking.CurrentNetns, "eth-test",
		netip.MustParsePrefix("192.168.124.2/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::2/64"))
	// "remote" simulates the remote host.
	l3.AddHost(t, "remote", "test-remote-host", "eth0",
		netip.MustParsePrefix("192.168.124.3/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::3/64"))
	// Add default routes to the "docker" Host from the "remote" Host.
	l3.Hosts["remote"].MustRun(t, "ip", "route", "add", "default", "via", "192.168.124.2")
	l3.Hosts["remote"].MustRun(t, "ip", "-6", "route", "add", "default", "via", "fdc0:36dc:a4dd::2")

	// Create a network and run a container on it.
	// Don't publish any ports.
	const netName = "test-acceptfwmark"
	network.CreateNoError(ctx, t, c, netName,
		network.WithOption(bridge.BridgeName, "br-acceptfwmark"),
		network.WithOption(bridge.TrustedHostInterfaces, "eth-test"),
		network.WithIPv6(),
	)
	t.Cleanup(func() {
		network.RemoveNoError(ctx, t, c, netName)
	})

	ctrId := container.Run(ctx, t, c,
		container.WithNetworkMode(netName),
		container.WithCmd("httpd", "-f"),
	)
	t.Cleanup(func() {
		c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})
	})

	insp := container.Inspect(ctx, t, c, ctrId)
	ctrIPv4 := insp.NetworkSettings.Networks[netName].IPAddress
	ctrIPv6 := insp.NetworkSettings.Networks[netName].GlobalIPv6Address

	const (
		httpSuccess = "404 Not Found"
		httpFail    = "Connection timed out"
		pingSuccess = 0
		pingFail    = 1
	)

	testPing := func(t *testing.T, cmd, addr string, expExit int) {
		t.Helper()
		t.Parallel()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			pingRes := icmd.RunCommand(cmd, "-n", "-c1", "-W3", addr)
			assert.Check(t, pingRes.ExitCode == expExit, "%s %s -> out:%s err:%s",
				cmd, addr, pingRes.Stdout(), pingRes.Stderr())
		})
	}
	testHttp := func(t *testing.T, addr, port, expOut string) {
		t.Helper()
		t.Parallel()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			u := "http://" + net.JoinHostPort(addr, port)
			res := icmd.RunCommand("curl", "--max-time", "3", "--show-error", "--silent", u)
			assert.Check(t, is.Contains(res.Combined(), expOut), "url:%s", u)
		})
	}

	test := func(name string, expPing int, expHttp string) {
		t.Run(name, func(t *testing.T) {
			t.Run("v4/ping", func(t *testing.T) {
				testPing(t, "ping", ctrIPv4.String(), expPing)
			})
			t.Run("v6/ping", func(t *testing.T) {
				testPing(t, "ping6", ctrIPv6.String(), expPing)
			})
			t.Run("v4/http", func(t *testing.T) {
				testHttp(t, ctrIPv4.String(), "80", expHttp)
			})
			t.Run("v6/http", func(t *testing.T) {
				testHttp(t, ctrIPv6.String(), "80", expHttp)
			})
		})
	}
	test("nofwmark", pingFail, httpFail)

	// This nftables will work if --firewall-backend=iptables, as long as it's iptables-nft.
	cmd := icmd.Command("nft", "-f", "-")
	res := icmd.RunCmd(cmd, icmd.WithStdin(strings.NewReader(`
		table inet test-acceptfwmark {
		  chain raw-PREROUTING {
			type filter hook prerouting priority raw
			iifname "eth-test" counter mark set 0xe
		  }
		}
	`)))
	res.Assert(t, icmd.Success)
	defer func() {
		icmd.RunCommand("nft", "delete table inet test-acceptfwmark").Assert(t, icmd.Success)
	}()

	test("fwmark", pingSuccess, httpSuccess)
}

// TestRoutedNonGateway checks whether a published container port on an endpoint in a
// gateway mode "routed" network is accessible when the routed network is not providing
// the container's default gateway.
func TestRoutedNonGateway(t *testing.T) {
	skip.If(t, testEnv.IsRootless())
	skip.If(t, networking.FirewalldRunning(), "Firewalld's IPv6_rpfilter=yes breaks IPv6 direct routing from L3Segment")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	c := d.NewClientT(t)
	defer c.Close()

	// Simulate the remote host.
	l3 := networking.NewL3Segment(t, "test-routed-open-ports",
		netip.MustParsePrefix("192.168.124.1/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::1/64"))
	defer l3.Destroy(t)
	// "docker" is the host where dockerd is running.
	const dockerHostIPv4 = "192.168.124.2"
	const dockerHostIPv6 = "fdc0:36dc:a4dd::2"
	l3.AddHost(t, "docker", networking.CurrentNetns, "eth-test",
		netip.MustParsePrefix(dockerHostIPv4+"/24"),
		netip.MustParsePrefix(dockerHostIPv6+"/64"))
	// "remote" simulates the remote host.
	l3.AddHost(t, "remote", "test-remote-host", "eth0",
		netip.MustParsePrefix("192.168.124.3/24"),
		netip.MustParsePrefix("fdc0:36dc:a4dd::3/64"))
	// Add default routes from the "remote" Host to the "docker" Host.
	l3.Hosts["remote"].MustRun(t, "ip", "route", "add", "default", "via", "192.168.124.2")
	l3.Hosts["remote"].MustRun(t, "ip", "-6", "route", "add", "default", "via", "fdc0:36dc:a4dd::2")

	// Create a dual-stack NAT'd network.
	const natNetName = "ds_nat"
	network.CreateNoError(ctx, t, c, natNetName,
		network.WithIPv6(),
		network.WithOption(bridge.BridgeName, natNetName),
	)
	defer network.RemoveNoError(ctx, t, c, natNetName)

	// Create a dual-stack routed network.
	const routedNetName = "ds_routed"
	network.CreateNoError(ctx, t, c, routedNetName,
		network.WithIPv6(),
		network.WithOption(bridge.BridgeName, routedNetName),
		network.WithOption(bridge.IPv4GatewayMode, "routed"),
		network.WithOption(bridge.IPv6GatewayMode, "routed"),
	)
	defer network.RemoveNoError(ctx, t, c, routedNetName)

	// Run a web server attached to both networks, and make sure the nat
	// network is selected as the gateway.
	ctrId := container.Run(ctx, t, c,
		container.WithCmd("httpd", "-f"),
		container.WithExposedPorts("80/tcp"),
		container.WithPortMap(networktypes.PortMap{
			networktypes.MustParsePort("80/tcp"): {{HostPort: "8080"}},
		}),
		container.WithNetworkMode(natNetName),
		container.WithNetworkMode(routedNetName),
		container.WithEndpointSettings(natNetName, &networktypes.EndpointSettings{GwPriority: 1}),
		container.WithEndpointSettings(routedNetName, &networktypes.EndpointSettings{GwPriority: 0}))
	defer container.Remove(ctx, t, c, ctrId, client.ContainerRemoveOptions{Force: true})

	testHttp := func(t *testing.T, addr, port, expOut string) {
		t.Helper()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			t.Parallel()
			u := "http://" + net.JoinHostPort(addr, port)
			res := icmd.RunCommand("curl", "--max-time", "3", "--show-error", "--silent", u)
			assert.Check(t, is.Contains(res.Combined(), expOut), "url:%s", u)
		})
	}

	const (
		httpSuccess = "404 Not Found"
		httpFail    = "Connection timed out"
	)

	insp := container.Inspect(ctx, t, c, ctrId)
	testcases := []struct {
		name    string
		addr    string
		port    string
		expHttp string
	}{
		{
			name:    "nat/published/v4",
			addr:    dockerHostIPv4,
			port:    "8080",
			expHttp: httpSuccess,
		},
		{
			name:    "nat/published/v6",
			addr:    dockerHostIPv6,
			port:    "8080",
			expHttp: httpSuccess,
		},
		{
			name:    "nat/direct/v4",
			addr:    insp.NetworkSettings.Networks[natNetName].IPAddress.String(),
			port:    "80",
			expHttp: httpFail,
		},
		{
			name:    "nat/direct/v6",
			addr:    insp.NetworkSettings.Networks[natNetName].GlobalIPv6Address.String(),
			port:    "80",
			expHttp: httpFail,
		},
		{
			name:    "routed/direct/v4",
			addr:    insp.NetworkSettings.Networks[routedNetName].IPAddress.String(),
			port:    "80",
			expHttp: httpSuccess,
		},
		{
			name:    "routed/direct/v6",
			addr:    insp.NetworkSettings.Networks[routedNetName].GlobalIPv6Address.String(),
			port:    "80",
			expHttp: httpSuccess,
		},
	}

	// Wrap parallel tests, otherwise defer statements run before tests finish.
	t.Run("w", func(t *testing.T) {
		for _, tc := range testcases {
			t.Run(tc.name, func(t *testing.T) {
				testHttp(t, tc.addr, tc.port, tc.expHttp)
			})
		}
	})
}

// TestAccessPublishedPortFromAnotherNetwork checks that a container can access
// ports published on the host by a container attached to a different network
// using both its gateway IP address, and the host IP address.
//
// Regression test for https://github.com/moby/moby/pull/49310.
func TestAccessPublishedPortFromAnotherNetwork(t *testing.T) {
	skip.If(t, testEnv.IsRootless, "rootlesskit maps ports on loopback in its own netns")

	ctx := setupTest(t)

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const servnet = "servnet"
	network.CreateNoError(ctx, t, c, servnet,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, servnet),
		network.WithIPv6(),
	)
	defer network.RemoveNoError(ctx, t, c, servnet)

	const clientnet = "clientnet"
	network.CreateNoError(ctx, t, c, clientnet,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, clientnet),
		network.WithIPv6(),
		network.WithIPAM("192.168.123.0/24", "192.168.123.1"),
		network.WithIPAM("fde5:4427:8b32::/64", "fde5:4427:8b32::1"),
	)
	defer network.RemoveNoError(ctx, t, c, clientnet)

	const (
		hostIPv4 = "10.0.128.2"
		hostIPv6 = "fd3f:69a1:3233::2"
	)

	defer enableIPv6OnAll(t)()
	// Add well-known addresses to the host.
	assert.NilError(t, exec.Command("ip", "addr", "add", hostIPv4+"/24", "dev", "eth0").Run())
	defer exec.Command("ip", "addr", "del", hostIPv4+"/24", "dev", "eth0").Run()
	assert.NilError(t, exec.Command("ip", "addr", "add", hostIPv6+"/64", "dev", "eth0").Run())
	defer exec.Command("ip", "addr", "del", hostIPv6+"/64", "dev", "eth0").Run()

	for _, tc := range []struct {
		name  string
		daddr string
	}{
		{
			name:  "IPv4/Gateway",
			daddr: "192.168.123.1",
		},
		{
			name:  "IPv6/Gateway",
			daddr: "fde5:4427:8b32::1",
		},
		{
			name:  "IPv4/Host",
			daddr: hostIPv4,
		},
		{
			name:  "IPv6/Host",
			daddr: hostIPv6,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// TODO: Figure out why is it flaky and fix the actual issue.
			// https://github.com/moby/moby/issues/49358
			retryFlaky(t, 5, func(t *testing.T) is.Comparison {
				serverID := container.Run(ctx, t, c,
					container.WithName("server"),
					container.WithCmd("nc", "-lp", "5000"),
					container.WithExposedPorts("5000/tcp"),
					container.WithPortMap(networktypes.PortMap{
						networktypes.MustParsePort("5000/tcp"): {{HostPort: "5000"}},
					}),
					container.WithNetworkMode(servnet))
				defer c.ContainerRemove(ctx, serverID, client.ContainerRemoveOptions{Force: true})

				clientID := container.Run(ctx, t, c,
					container.WithName("client"),
					container.WithCmd("/bin/sh", "-c", fmt.Sprintf("echo foobar | nc -w1 %s 5000", tc.daddr)),
					container.WithNetworkMode(clientnet))
				defer c.ContainerRemove(ctx, clientID, client.ContainerRemoveOptions{Force: true})

				logs := getContainerStdout(t, ctx, c, serverID)
				return is.Contains(logs, "foobar")
			})
		})
	}
}

func retryFlaky(t *testing.T, retries int, f func(t *testing.T) is.Comparison) {
	for i := 0; i < retries-1; i++ {
		comp := f(t)
		if comp().Success() {
			return
		}
		t.Log("Retrying...")
		time.Sleep(time.Second)
	}

	assert.Assert(t, f(t))
}

// TestDirectRemoteAccessOnExposedPort checks that remote hosts can't directly
// reach a container on one of its exposed port. That is, if container has the
// IP address 172.17.24.2, and its port 443 is exposed on the host, no remote
// host should be able to reach 172.17.24.2:443 directly.
//
// Regression test for https://github.com/moby/moby/issues/45610.
func TestDirectRemoteAccessOnExposedPort(t *testing.T) {
	// This test checks iptables rules that live in dockerd's netns. In the case
	// of rootlesskit, this is not the same netns as the host, so they don't
	// have any effect.
	// TODO(aker): we need to figure out what we want to do for rootlesskit.
	// skip.If(t, testEnv.IsRootless, "rootlesskit has its own netns")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)
	testDirectRemoteAccessOnExposedPort(t, ctx, d, false)
}

// TestAllowDirectRemoteAccessOnExposedPort checks that remote hosts can directly
// reach a container on one of its exposed ports - if the daemon is running with
// option --allow-direct-routing.
func TestAllowDirectRemoteAccessOnExposedPort(t *testing.T) {
	// This test checks iptables rules that live in dockerd's netns. In the case
	// of rootlesskit, this is not the same netns as the host, so they don't
	// have any effect.
	// TODO(aker): we need to figure out what we want to do for rootlesskit.
	// skip.If(t, testEnv.IsRootless, "rootlesskit has its own netns")

	ctx := setupTest(t)
	d := daemon.New(t)
	d.StartWithBusybox(ctx, t, "--allow-direct-routing")
	defer d.Stop(t)
	testDirectRemoteAccessOnExposedPort(t, ctx, d, true)
}

func testDirectRemoteAccessOnExposedPort(t *testing.T, ctx context.Context, d *daemon.Daemon, allowDirectRouting bool) {
	const (
		hostIPv4 = "192.168.120.2"
		hostIPv6 = "fdbc:277b:d40b::2"
	)

	l3 := networking.NewL3Segment(t, "test-direct-remote-access",
		netip.MustParsePrefix("192.168.120.1/24"),
		netip.MustParsePrefix("fdbc:277b:d40b::1/64"))
	defer l3.Destroy(t)
	// "docker" is the host where dockerd is running.
	const hostIfName = "test-eth"
	l3.AddHost(t, "docker", networking.CurrentNetns, hostIfName,
		netip.MustParsePrefix(hostIPv4+"/24"),
		netip.MustParsePrefix(hostIPv6+"/64"))
	l3.AddHost(t, "attacker", "test-direct-remote-access-attacker", "eth0",
		netip.MustParsePrefix("192.168.120.3/24"),
		netip.MustParsePrefix("fdbc:277b:d40b::3/64"))

	c := d.NewClientT(t)
	defer c.Close()
	for _, tc := range []struct {
		name         string
		gwMode       string
		gwAddr       netip.Prefix
		ipv4Disabled bool
		ipv6Disabled bool
		trusted      bool
	}{
		{
			name:   "NAT/IPv4",
			gwMode: "nat",
			gwAddr: netip.MustParsePrefix("172.24.10.1/24"),
		},
		{
			name:   "NAT/IPv6",
			gwMode: "nat",
			gwAddr: netip.MustParsePrefix("fda9:a651:db6d::1/64"),
		},
		{
			name:    "NAT/IPv4/trusted",
			gwMode:  "nat",
			gwAddr:  netip.MustParsePrefix("172.24.10.1/24"),
			trusted: true,
		},
		{
			name:    "NAT/IPv6/trusted",
			gwMode:  "nat",
			gwAddr:  netip.MustParsePrefix("fda9:a651:db6d::1/64"),
			trusted: true,
		},
		{
			name:   "NAT unprotected/IPv4",
			gwMode: "nat-unprotected",
			gwAddr: netip.MustParsePrefix("172.24.10.1/24"),
		},
		{
			name:   "NAT unprotected/IPv6",
			gwMode: "nat-unprotected",
			gwAddr: netip.MustParsePrefix("fda9:a651:db6d::1/64"),
		},
		{
			name:         "Proxy/IPv4",
			gwMode:       "nat",
			gwAddr:       netip.MustParsePrefix("fd05:b021:403f::1/64"),
			ipv4Disabled: true,
		},
		{
			name:         "Proxy/IPv6",
			gwMode:       "nat",
			gwAddr:       netip.MustParsePrefix("172.24.11.1/24"),
			ipv6Disabled: true,
		},
		{
			name:   "Routed/IPv4",
			gwMode: "routed",
			gwAddr: netip.MustParsePrefix("172.24.12.1/24"),
		},
		{
			name:   "Routed/IPv6",
			gwMode: "routed",
			gwAddr: netip.MustParsePrefix("fd82:5787:b217::1/64"),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expDirectAccess := tc.gwMode == "routed" || tc.gwMode == "nat-unprotected" || tc.trusted || allowDirectRouting
			skip.If(t, expDirectAccess && testEnv.IsRootless(), "rootlesskit doesn't support routed mode as it's running in a separate netns")

			testutil.StartSpan(ctx, t)

			nwOpts := []func(*client.NetworkCreateOptions){
				network.WithIPAM(tc.gwAddr.Masked().String(), tc.gwAddr.Addr().String()),
				network.WithOption(bridge.IPv4GatewayMode, tc.gwMode),
				network.WithOption(bridge.IPv6GatewayMode, tc.gwMode),
			}

			if tc.ipv4Disabled {
				nwOpts = append(nwOpts, network.WithIPv4Disabled())
			}
			if tc.ipv6Disabled {
				nwOpts = append(nwOpts, network.WithIPv6Disabled())
			}
			if tc.gwAddr.Addr().Is6() {
				nwOpts = append(nwOpts, network.WithIPv6())
			}
			if tc.trusted {
				nwOpts = append(nwOpts, network.WithOption(bridge.TrustedHostInterfaces, hostIfName))
			}

			const bridgeName = "brattacked"
			network.CreateNoError(ctx, t, c, bridgeName, append(nwOpts,
				network.WithDriver("bridge"),
				network.WithOption(bridge.BridgeName, bridgeName),
			)...)
			defer network.RemoveNoError(ctx, t, c, bridgeName)

			const hostPort = "5000"
			hostIP := hostIPv4
			if tc.gwAddr.Addr().Is6() {
				hostIP = hostIPv6
			}

			ctrIP := tc.gwAddr.Addr().Next()

			test := func(t *testing.T, host networking.Host, daddr, payload string) bool {
				serverID := container.Run(ctx, t, c,
					container.WithName(sanitizeCtrName(t.Name()+"-server")),
					container.WithCmd("nc", "-lup", "5000"),
					container.WithExposedPorts("5000/udp"),
					container.WithPortMap(networktypes.PortMap{
						networktypes.MustParsePort("5000/udp"): {{HostPort: hostPort}},
					}),
					container.WithNetworkMode(bridgeName),
					container.WithEndpointSettings(bridgeName, &networktypes.EndpointSettings{
						IPAddress:   ctrIP,
						IPPrefixLen: ctrIP.BitLen(),
					}))
				defer c.ContainerRemove(ctx, serverID, client.ContainerRemoveOptions{Force: true})

				return sendPayloadFromHost(t, host, daddr, hostPort, payload, func() bool {
					logs := getContainerStdout(t, ctx, c, serverID)
					return strings.Contains(logs, payload)
				})
			}

			if tc.gwMode != "routed" {
				// Send a payload to the port mapped on the host -- this should work
				res := test(t, l3.Hosts["attacker"], hostIP, "foobar")
				assert.Assert(t, res, "Remote host should have access to port published on the host, but no payload was received by the container")
			}

			// Now send a payload directly to the container. With gw_mode=routed,
			// this should work. With gw_mode=nat, this should fail.
			l3.Hosts["attacker"].Run(t, "ip", "route", "add", tc.gwAddr.Masked().String(), "via", hostIP, "dev", "eth0")
			defer l3.Hosts["attacker"].Run(t, "ip", "route", "delete", tc.gwAddr.Masked().String(), "via", hostIP, "dev", "eth0")

			res := test(t, l3.Hosts["attacker"], ctrIP.String(), "bar baz")
			if expDirectAccess {
				assert.Assert(t, res, "Remote host should have direct access to the published port, but no payload was received by the container")
			} else {
				assert.Assert(t, !res, "Remote host should not have direct access to the published port, but payload was received by the container")
			}
		})
	}
}

// TestAccessPortPublishedOnLoopbackAddress checks that ports published on
// loopback addresses can't be accessed by remote hosts.
//
// Regression test for https://github.com/moby/moby/issues/45610.
func TestAccessPortPublishedOnLoopbackAddress(t *testing.T) {
	// rootlesskit uses a proxy to forward ports from the host netns to its own
	// netns, so it's not affected by the original issue.
	skip.If(t, testEnv.IsRootless, "rootlesskit has its own netns")

	ctx := setupTest(t)

	l3 := networking.NewL3Segment(t, "test-access-loopback",
		netip.MustParsePrefix("192.168.121.1/24"))
	defer l3.Destroy(t)
	// "docker" is the host where dockerd is running.
	l3.AddHost(t, "docker", networking.CurrentNetns, "eth-test",
		netip.MustParsePrefix("192.168.121.2/24"))
	l3.AddHost(t, "attacker", "test-access-loopback-attacker", "eth0",
		netip.MustParsePrefix("192.168.121.3/24"))

	d := daemon.New(t)
	d.StartWithBusybox(ctx, t)
	defer d.Stop(t)

	c := d.NewClientT(t)
	defer c.Close()

	const bridgeName = "brtest"
	network.CreateNoError(ctx, t, c, bridgeName,
		network.WithDriver("bridge"),
		network.WithOption(bridge.BridgeName, bridgeName),
	)
	defer network.RemoveNoError(ctx, t, c, bridgeName)

	const (
		loIP     = "127.0.0.2"
		hostPort = "5000"
	)

	// The busybox version of netcat doesn't handle properly the `-k` flag,
	// which should allow it to print the payload of multiple sequential
	// connections. To overcome that limitation, start a new container every
	// time we want to test if a payload is received.
	test := func(t *testing.T, host networking.Host, payload string) bool {
		t.Helper()

		serverID := container.Run(ctx, t, c,
			container.WithName("server"),
			container.WithCmd("nc", "-lup", "5000"),
			container.WithExposedPorts("5000/udp"),
			// This port is mapped on 127.0.0.2, so it should not be remotely accessible.
			container.WithPortMap(networktypes.PortMap{
				networktypes.MustParsePort("5000/udp"): {{HostIP: netip.MustParseAddr(loIP), HostPort: hostPort}},
			}),
			container.WithNetworkMode(bridgeName))
		defer c.ContainerRemove(ctx, serverID, client.ContainerRemoveOptions{Force: true})

		return sendPayloadFromHost(t, host, loIP, hostPort, payload, func() bool {
			logs := getContainerStdout(t, ctx, c, serverID)
			return strings.Contains(logs, payload)
		})
	}

	// Check if the local host has access to the published port.
	res := test(t, l3.Hosts["docker"], "foobar")
	assert.Assert(t, res, "Local host should have access to the published port, but no payload was received by the container")

	// Add a route to the loopback address on the attacker host in order to
	// conduct the attack scenario.
	l3.Hosts["attacker"].Run(t, "ip", "route", "add", loIP+"/32", "via", "192.168.121.2", "dev", "eth0")
	defer l3.Hosts["attacker"].Run(t, "ip", "route", "delete", loIP+"/32", "via", "192.168.121.2", "dev", "eth0")

	// Check that remote access to the loopback address is correctly blocked.
	res = test(t, l3.Hosts["attacker"], "bar baz")
	assert.Assert(t, !res, "Remote host should not have access to the published port, but the payload was received by the container")
}

// Send a payload to daddr:dport a few times from the 'host' netns. Stop
// sending payloads when 'check' returns true. Return the result of 'check'.
//
// UDP is preferred here as it's a one-way, connectionless protocol. With TCP
// the three-way handshake has to be completed before sending a payload, but
// since some test cases try to spoof the loopback address, the 'attacker host'
// will drop the SYN-ACK by default (because the source addr will be considered
// invalid / non-routable). This would require further tuning to make it work.
// With UDP, this problem doesn't exist - the payload can be sent straight away.
// But UDP is inherently unreliable, so we need to send the payload multiple
// times.
func sendPayloadFromHost(t *testing.T, host networking.Host, daddr, dport, payload string, check func() bool) bool {
	var res bool
	host.Do(t, func() {
		for i := range 10 {
			t.Logf("Sending probe #%d to %s:%s from host %s", i, daddr, dport, host.Name)
			icmd.RunCommand("/bin/sh", "-c", fmt.Sprintf("echo '%s' | nc -w1 -u %s %s", payload, daddr, dport)).Assert(t, icmd.Success)

			res = check()
			if res {
				return
			}

			time.Sleep(50 * time.Millisecond)
		}
	})
	return res
}

func getContainerStdout(t *testing.T, ctx context.Context, c *client.Client, ctrID string) string {
	logReader, err := c.ContainerLogs(ctx, ctrID, client.ContainerLogsOptions{
		ShowStdout: true,
	})
	assert.NilError(t, err)
	defer logReader.Close()

	var logs bytes.Buffer
	_, err = stdcopy.StdCopy(&logs, nil, logReader)
	assert.NilError(t, err)

	return logs.String()
}

// TestSkipRawRules checks that when env var DOCKER_INSECURE_NO_IPTABLES_RAW=1, no rules are added to
// the iptables "raw" table - as a workaround for kernels that don't have CONFIG_IP_NF_RAW.
// See https://github.com/moby/moby/issues/49557
func TestSkipRawRules(t *testing.T) {
	skip.If(t, networking.FirewalldRunning(), "can't use firewalld in host netns to add rules in L3Segment")
	skip.If(t, !strings.Contains(testEnv.FirewallBackendDriver(), "iptables"),
		"test is iptables specific, and iptables isn't in use")
	skip.If(t, testEnv.IsRootless, "can't use L3Segment, or check iptables rules")

	testcases := []struct {
		name          string
		noIptablesRaw string
	}{
		{
			name:          "skip=false",
			noIptablesRaw: "0",
		},
		{
			name:          "skip=true",
			noIptablesRaw: "1",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			// Run in a new netns, to make sure there are no raw rules left behind by other tests
			const l3SegHost = "skip-raw"
			l3 := networking.NewL3Segment(t, "test-"+l3SegHost)
			defer l3.Destroy(t)
			hostAddrs := []netip.Prefix{
				netip.MustParsePrefix("192.168.234.0/24"),
				netip.MustParsePrefix("fd3f:c09d:715b::/64"),
			}
			l3.AddHost(t, l3SegHost, "ns-"+l3SegHost, "eth0", hostAddrs...)
			l3.Hosts[l3SegHost].Do(t, func() {
				ctx := setupTest(t)
				d := daemon.New(t, daemon.WithEnvVars("DOCKER_INSECURE_NO_IPTABLES_RAW="+tc.noIptablesRaw))
				d.StartWithBusybox(ctx, t, "--ipv6", "--bip=192.168.0.1/24", "--bip6=fd30:1159:a755::1/64")
				defer d.Stop(t)
				c := d.NewClientT(t)
				defer c.Close()

				ctrId := container.Run(ctx, t, c,
					container.WithExposedPorts("80/tcp"),
					container.WithPortMap(networktypes.PortMap{networktypes.MustParsePort("80/tcp"): {
						{HostIP: netip.MustParseAddr("127.0.0.1"), HostPort: "8080"},
						{HostPort: "8081"},
					}}),
				)
				defer c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})

				res4 := icmd.RunCommand("iptables", "-S", "-t", "raw")
				golden.Assert(t, res4.Stdout(), t.Name()+"_ipv4.golden")
				res6 := icmd.RunCommand("ip6tables", "-S", "-t", "raw")
				golden.Assert(t, res6.Stdout(), t.Name()+"_ipv6.golden")
			})
		})
	}
}

// Regression test for https://github.com/docker/compose/issues/12846
func TestMixAnyWithSpecificHostAddrs(t *testing.T) {
	ctx := setupTest(t)

	for _, proto := range []string{"tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			// Start a new daemon, so the port allocator will start with new/empty ephemeral port ranges,
			// making a clash more likely.
			d := daemon.New(t)
			d.StartWithBusybox(ctx, t)
			defer d.Stop(t)
			c := d.NewClientT(t)
			defer c.Close()

			ctrId := container.Run(ctx, t, c,
				container.WithExposedPorts("80/"+proto, "81/"+proto, "82/"+proto),
				container.WithPortMap(networktypes.PortMap{
					networktypes.MustParsePort("81/" + proto): {{}},
					networktypes.MustParsePort("82/" + proto): {{}},
					networktypes.MustParsePort("80/" + proto): {{HostIP: netip.MustParseAddr("127.0.0.1")}},
				}),
			)
			defer c.ContainerRemove(ctx, ctrId, client.ContainerRemoveOptions{Force: true})

			insp := container.Inspect(ctx, t, c, ctrId)
			hostPorts := map[string]struct{}{}
			for cp, hps := range insp.NetworkSettings.Ports {
				// Check each of the container ports is mapped to a different host port.
				p := hps[0].HostPort
				if _, ok := hostPorts[p]; ok {
					t.Errorf("host port %s is mapped to different container ports: %v", p, insp.NetworkSettings.Ports)
				}
				hostPorts[p] = struct{}{}

				// For this container port, check the same host port is mapped for each host address (0.0.0.0 and ::).
				for _, hp := range hps {
					assert.Check(t, p == hp.HostPort, "container port %d is mapped to different host ports: %v", cp, hps)
				}
			}
		})
	}
}
