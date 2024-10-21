package networking

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
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
		expPortMap nat.PortMap
	}{
		{
			name: "defaults",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
					{HostIP: "::", HostPort: "8080"},
				},
			},
		},
		{
			name:    "nat4 routed6",
			gwMode4: "nat",
			gwMode6: "routed",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: "8080"},
					{HostIP: "::", HostPort: ""},
				},
			},
		},
		{
			name:    "nat6 routed4",
			gwMode4: "routed",
			gwMode6: "nat",
			expPortMap: nat.PortMap{
				"80/tcp": []nat.PortBinding{
					{HostIP: "0.0.0.0", HostPort: ""},
					{HostIP: "::", HostPort: "8080"},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := testutil.StartSpan(ctx, t)

			const netName = "testnet"
			nwOpts := []func(options *networktypes.CreateOptions){
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
				container.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "8080"}}}),
			)
			defer c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})

			inspect := container.Inspect(ctx, t, c, id)
			assert.Check(t, is.DeepEqual(inspect.NetworkSettings.Ports, tc.expPortMap))
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
		container.WithPortMap(nat.PortMap{"80": {{HostIP: "0.0.0.0"}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports["80/tcp"][0].HostPort

	clientCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	res := container.RunAttach(clientCtx, t, c,
		container.WithNetworkMode(clientNetName),
		container.WithCmd("wget", "http://"+hostAddr+":"+hostPort),
	)
	defer c.ContainerRemove(ctx, res.ContainerID, containertypes.RemoveOptions{Force: true})
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
		container.WithPortMap(nat.PortMap{"54/udp": {{HostIP: "0.0.0.0"}}}),
		container.WithCmd("/bin/sh", "-c", "echo 'foobar.internal 192.168.155.23' | dnsd -c - -p 54"),
	)
	defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports["54/udp"][0].HostPort

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
		container.WithPortMap(nat.PortMap{"80": {{HostIP: "::1"}}}),
		container.WithCmd("httpd", "-f"),
	)
	defer c.ContainerRemove(ctx, serverId, containertypes.RemoveOptions{Force: true})

	inspect := container.Inspect(ctx, t, c, serverId)
	hostPort := inspect.NetworkSettings.Ports["80/tcp"][0].HostPort

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

	sysctls := strings.Split(string(out), "\n")
	for _, sysctl := range sysctls {
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
	defer assert.NilError(t, exec.Command("ip", "addr", "del", "fdfb:5cbb:29bf::2/64", "dev", "eth0").Run())

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
			bridgeOpts := []func(options *networktypes.CreateOptions){
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
				container.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: hostPort}}}),
				container.WithCmd("httpd", "-f"),
				container.WithNetworkMode(bridgeName))
			defer c.ContainerRemove(ctx, serverID, containertypes.RemoveOptions{Force: true})

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
		container.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: hostPort}}}),
		container.WithCmd("httpd", "-f"),
		container.WithNetworkMode(bridgeName))
	defer c.ContainerRemove(ctx, serverID, containertypes.RemoveOptions{Force: true})

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
		container.WithPortMap(nat.PortMap{"80/tcp": {{HostPort: "1780"}}}),
		container.WithCmd("httpd", "-f"),
		container.WithNetworkMode(netName),
	}

	container.Run(ctx, t, c, ctrOpts...)
	defer c.ContainerRemove(ctx, ctrName, containertypes.RemoveOptions{Force: true})

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
	assert.NilError(t, c.ContainerRemove(ctx, ctrName, containertypes.RemoveOptions{Force: true}))

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
	l3.Hosts["remote"].Run(t, "ip", "route", "add", "default", "via", "192.168.124.2")
	l3.Hosts["remote"].Run(t, "ip", "-6", "route", "add", "default", "via", "fdc0:36dc:a4dd::2")

	type ctrDesc struct {
		id   string
		ipv4 string
		ipv6 string
	}

	// Create a network and run a container on it.
	// Run http servers on ports 80 and 81, but only map/open port 80.
	createNet := func(gwMode string) ctrDesc {
		netName := "test-" + gwMode
		network.CreateNoError(ctx, t, c, netName,
			network.WithDriver("bridge"),
			network.WithIPv6(),
			network.WithOption(bridge.BridgeName, "br-"+gwMode),
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
			container.WithPortMap(nat.PortMap{"80/tcp": {}}),
		)
		t.Cleanup(func() {
			c.ContainerRemove(ctx, ctrId, containertypes.RemoveOptions{Force: true})
		})

		container.ExecT(ctx, t, c, ctrId, []string{"httpd", "-p", "80"})
		container.ExecT(ctx, t, c, ctrId, []string{"httpd", "-p", "81"})

		insp := container.Inspect(ctx, t, c, ctrId)
		return ctrDesc{
			id:   ctrId,
			ipv4: insp.NetworkSettings.Networks[netName].IPAddress,
			ipv6: insp.NetworkSettings.Networks[netName].GlobalIPv6Address,
		}
	}

	const (
		httpSuccess = "404 Not Found"
		httpFail    = "Connection timed out"
		pingSuccess = 0
		pingFail    = 1
	)

	networks := map[string]ctrDesc{
		"nat":    createNet("nat"),
		"routed": createNet("routed"),
	}
	expPingExit := map[string]int{
		"nat":    pingFail,
		"routed": pingSuccess,
	}

	testPing := func(t *testing.T, cmd, addr string, expExit int) {
		t.Helper()
		t.Parallel()
		l3.Hosts["remote"].Do(t, func() {
			t.Helper()
			pingRes := icmd.RunCommand(cmd, "--numeric", "--count=1", "--timeout=3", addr)
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
	for _, fwdPolicy := range []string{"ACCEPT", "DROP"} {
		networking.SetFilterForwardPolicies(t, fwdPolicy)
		t.Run(fwdPolicy, func(t *testing.T) {
			for _, gwMode := range []string{"nat", "routed"} {
				t.Run(gwMode+"/v4/ping", func(t *testing.T) {
					testPing(t, "ping", networks[gwMode].ipv4, expPingExit[gwMode])
				})
				t.Run(gwMode+"/v6/ping", func(t *testing.T) {
					testPing(t, "ping6", networks[gwMode].ipv6, expPingExit[gwMode])
				})
				t.Run(gwMode+"/v4/http/80", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv4, "80", httpSuccess)
				})
				t.Run(gwMode+"/v4/http/81", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv4, "81", httpFail)
				})
				t.Run(gwMode+"/v6/http/80", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv6, "80", httpSuccess)
				})
				t.Run(gwMode+"/v6/http/81", func(t *testing.T) {
					testHttp(t, networks[gwMode].ipv6, "81", httpFail)
				})
			}
		})
	}
}
