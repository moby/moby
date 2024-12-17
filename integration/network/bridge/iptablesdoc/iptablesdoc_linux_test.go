// Package iptablesdoc runs docker, creates networks, runs containers and
// captures iptables output for various configurations.
//
// The iptables output is then used with a markdown text/template from the
// "templates" directory for each configuration (for each "section" in "index"),
// to generate a markdown document for each section.
//
// The newly generated documents are placed in:
//
//	bundles/test-integration/TestBridgeIptablesDoc/iptables.md
//
// If the generated doc differs from the "golden" reference in "generated/",
// the test fails. When that happens:
//
//   - check the iptables rules changes in the diff
//   - update the description in the corresponding "_templ.md" file
//   - re-run with TESTFLAGS='-update' to update the reference docs
package iptablesdoc

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"text/template"
	"time"

	containertypes "github.com/docker/docker/api/types/container"
	networktypes "github.com/docker/docker/api/types/network"
	swarmtypes "github.com/docker/docker/api/types/swarm"
	"github.com/docker/docker/client"
	"github.com/docker/docker/integration/internal/container"
	"github.com/docker/docker/integration/internal/network"
	"github.com/docker/docker/internal/testutils/networking"
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/testutil"
	"github.com/docker/docker/testutil/daemon"
	"github.com/docker/go-connections/nat"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/poll"
	"gotest.tools/v3/skip"
)

var (
	docNetworks = []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}
	docGateways = []string{"192.0.2.1", "198.51.100.1", "203.0.113.1"}
)

type ctrDesc struct {
	name         string
	portMappings nat.PortMap
}

type networkDesc struct {
	name       string
	gwMode     string
	noICC      bool
	internal   bool
	containers []ctrDesc
}

type section struct {
	name            string
	noUserlandProxy bool
	swarm           bool
	networks        []networkDesc
}

var index = []section{
	{
		name: "new-daemon.md",
	},
	{
		name: "usernet-portmap.md",
		networks: []networkDesc{{
			name: "bridge1",
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name:            "usernet-portmap-noproxy.md",
		noUserlandProxy: true,
		networks: []networkDesc{{
			name: "bridge1",
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name: "usernet-portmap-noicc.md",
		networks: []networkDesc{{
			name:  "bridge1",
			noICC: true,
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name: "usernet-internal.md",
		networks: []networkDesc{{
			name:     "bridgeICC",
			internal: true,
			containers: []ctrDesc{
				{
					name: "c1",
				},
			},
		}, {
			name:     "bridgeNoICC",
			internal: true,
			noICC:    true,
			containers: []ctrDesc{
				{
					name: "c1",
				},
			},
		}},
	},
	{
		name: "usernet-portmap-routed.md",
		networks: []networkDesc{{
			name:   "bridge1",
			gwMode: "routed",
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name: "usernet-portmap-natunprot.md",
		networks: []networkDesc{{
			name:   "bridge1",
			gwMode: "nat-unprotected",
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name:  "swarm-portmap.md",
		swarm: true,
		networks: []networkDesc{{
			containers: []ctrDesc{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
}

// iptCmdType is used to look up iptCmds in the markdown (can't use an int
// type, or a new string type, so it's just an alias).
type iptCmdType = string

const (
	iptCmdLFilter4        iptCmdType = "LFilter4"
	iptCmdSFilter4        iptCmdType = "SFilter4"
	iptCmdLFilterDocker4  iptCmdType = "LFilterDocker4"
	iptCmdSFilterForward4 iptCmdType = "SFilterForward4"
	iptCmdSFilterDocker4  iptCmdType = "SFilterDocker4"
	iptCmdLNat4           iptCmdType = "LNat4"
	iptCmdSNat4           iptCmdType = "SNat4"
)

var iptCmds = map[iptCmdType][]string{
	iptCmdLFilter4:        {"iptables", "-nvL", "--line-numbers", "-t", "filter"},
	iptCmdSFilter4:        {"iptables", "-S", "-t", "filter"},
	iptCmdSFilterForward4: {"iptables", "-S", "FORWARD"},
	iptCmdLFilterDocker4:  {"iptables", "-nvL", "DOCKER", "--line-numbers", "-t", "filter"},
	iptCmdSFilterDocker4:  {"iptables", "-S", "DOCKER"},
	iptCmdLNat4:           {"iptables", "-nvL", "--line-numbers", "-t", "nat"},
	iptCmdSNat4:           {"iptables", "-S", "-t", "nat"},
}

func TestBridgeIptablesDoc(t *testing.T) {
	skip.If(t, testEnv.IsRootless)
	ctx := setupTest(t)

	// Get the full path for "bundles/TestBridgeIptablesDoc".
	dest := os.Getenv("DOCKER_INTEGRATION_DAEMON_DEST")
	if dest == "" {
		dest = os.Getenv("DEST")
	}
	dest = filepath.Join(dest, t.Name())

	// Set up an L3Segment, which will have a netns for each "section".
	addr4 := netip.MustParseAddr("192.168.124.1")
	addr6 := netip.MustParseAddr("fdc0:36dc:a4dd::1")
	l3 := networking.NewL3Segment(t, "gen-iptables-doc",
		netip.PrefixFrom(addr4, 24),
		netip.PrefixFrom(addr6, 64),
	)
	t.Cleanup(func() { l3.Destroy(t) })

	for i, sec := range index {
		// Create a netns for this section.
		addr4 = addr4.Next()
		addr6 = addr6.Next()
		hostname := fmt.Sprintf("docker%d", i)
		l3.AddHost(t, hostname, hostname+"-host", "eth0",
			netip.PrefixFrom(addr4, 24),
			netip.PrefixFrom(addr6, 64),
		)
		host := l3.Hosts[hostname]
		// Stop the interface, to reduce the chances of stray packets getting counted by iptables.
		host.MustRun(t, "ip", "link", "set", "eth0", "down")

		t.Run("gen_"+sec.name, func(t *testing.T) {
			// t.Parallel() - doesn't speed things up, startup times just extend
			runTestNet(t, testutil.StartSpan(ctx, t), dest, sec, host)
		})
	}
}

func runTestNet(t *testing.T, ctx context.Context, bundlesDir string, section section, host networking.Host) {
	var dArgs []string
	if section.noUserlandProxy {
		dArgs = append(dArgs, "--userland-proxy=false")
	}
	if section.swarm {
		if _, err := netlink.GenlFamilyGet("IPVS"); err != nil {
			t.Skipf("No IPVS, so DOCKER-INGRESS will not be set up: %v", err)
		}
		dArgs = append(dArgs, "--swarm-default-advertise-addr="+host.Iface)
	}

	// Start the daemon in its own network namespace.
	var d *daemon.Daemon
	host.Do(t, func() {
		// Run without OTEL because there's no routing from this netns for it - which
		// means the daemon doesn't shut down cleanly, causing the test to fail.
		d = daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
		d.StartWithBusybox(ctx, t, dArgs...)
		t.Cleanup(func() { d.Stop(t) })
	})

	assert.Assert(t, len(section.networks) < len(docNetworks), "Don't have enough container network addresses")
	if section.swarm {
		d.SwarmInit(ctx, t, swarmtypes.InitRequest{})
		createServices(ctx, t, d, section, host)
	} else {
		createBridgeNetworks(ctx, t, d, section)
	}

	iptablesOutput := runIptables(t, host)
	generated := generate(t, section.name, iptablesOutput)

	// Write the output to the 'bundles' directory for easy reference.
	outFile := filepath.Join(bundlesDir, section.name)
	err := os.WriteFile(outFile, []byte(generated), 0o644)
	assert.NilError(t, err)
	t.Log("Wrote ", outFile)

	// Compare against "golden" results.
	// Use full path so that the directory containing generated docs doesn't
	// have to be called 'testdata'.
	wd, err := os.Getwd()
	assert.NilError(t, err)
	golden.Assert(t, generated, filepath.Join(wd, "generated", section.name))
}

func createBridgeNetworks(ctx context.Context, t *testing.T, d *daemon.Daemon, section section) {
	c := d.NewClientT(t)
	defer c.Close()

	for i, nw := range section.networks {
		gwMode := nw.gwMode
		if gwMode == "" {
			gwMode = "nat"
		}
		netOpts := []func(*networktypes.CreateOptions){
			network.WithIPAM(docNetworks[i], docGateways[i]),
			network.WithOption(bridge.BridgeName, nw.name),
			network.WithOption(bridge.IPv4GatewayMode, gwMode),
		}
		if nw.noICC {
			netOpts = append(netOpts, network.WithOption(bridge.EnableICC, "false"))
		}
		if nw.internal {
			netOpts = append(netOpts, network.WithInternal())
		}
		network.CreateNoError(ctx, t, c, nw.name, netOpts...)
		t.Cleanup(func() { network.RemoveNoError(ctx, t, c, nw.name) })

		for _, ctr := range nw.containers {
			var exposedPorts []string
			for ep := range ctr.portMappings {
				exposedPorts = append(exposedPorts, ep.Port()+"/"+ep.Proto())
			}
			id := container.Run(ctx, t, c,
				container.WithNetworkMode(nw.name),
				container.WithExposedPorts(exposedPorts...),
				container.WithPortMap(ctr.portMappings),
			)
			t.Cleanup(func() {
				c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
			})
		}
	}
}

func createServices(ctx context.Context, t *testing.T, d *daemon.Daemon, section section, host networking.Host) {
	c := d.NewClientT(t)
	defer c.Close()

	for _, nw := range section.networks {
		for _, ctr := range nw.containers {
			// Convert portMap to swarm PortConfig, just well-enough for this test.
			var portConfig []swarmtypes.PortConfig
			for ctrPP, hostPorts := range ctr.portMappings {
				for _, hostPort := range hostPorts {
					hp, err := strconv.Atoi(hostPort.HostPort)
					assert.NilError(t, err)
					portConfig = append(portConfig, swarmtypes.PortConfig{
						Protocol:      swarmtypes.PortConfigProtocol(ctrPP.Proto()),
						PublishedPort: uint32(hp),
						TargetPort:    uint32(ctrPP.Int()),
					})
				}
			}
			id := d.CreateService(ctx, t, func(s *swarmtypes.Service) {
				s.Spec = swarmtypes.ServiceSpec{
					TaskTemplate: swarmtypes.TaskSpec{
						ContainerSpec: &swarmtypes.ContainerSpec{
							Image:   "busybox:latest",
							Command: []string{"/bin/top"},
						},
					},
					EndpointSpec: &swarmtypes.EndpointSpec{
						Ports: portConfig,
					},
				}
			})
			t.Cleanup(func() { d.RemoveService(ctx, t, id) })
			poll.WaitOn(t, func(_ poll.LogT) poll.Result {
				return pollService(ctx, t, c, host)
			}, poll.WithTimeout(10*time.Second), poll.WithDelay(100*time.Millisecond))
		}
	}
}

func pollService(ctx context.Context, t *testing.T, c *client.Client, host networking.Host) poll.Result {
	cl, err := c.ContainerList(ctx, containertypes.ListOptions{})
	if err != nil {
		return poll.Error(fmt.Errorf("failed to list containers: %w", err))
	}
	if len(cl) != 1 {
		return poll.Continue("got %d containers, want 1", len(cl))
	}
	// The DOCKER-INGRESS chain seems to be created, then populated, a few
	// milliseconds after the container starts. So, also wait for a conntrack
	// "RELATED" rule to appear in the chain.
	// TODO(robmry) - is there something better to poll?
	di, err := host.Run(t, "iptables", "-L", "DOCKER-INGRESS")
	if err != nil || !strings.Contains(di, "RELATED") {
		return poll.Continue("ingress chain not ready, got: %s", di)
	}
	return poll.Success()
}

var rePacketByteCounts = regexp.MustCompile(`\d+ packets, \d+ bytes`)

func runIptables(t *testing.T, host networking.Host) map[iptCmdType]string {
	host.MustRun(t, "iptables", "-Z")
	host.MustRun(t, "iptables", "-Z", "-t", "nat")
	res := map[iptCmdType]string{}
	for k, cmd := range iptCmds {
		d := host.MustRun(t, cmd[0], cmd[1:]...)
		// In CI, the OUTPUT chain sometimes sees a packet. Remove the counts.
		d = rePacketByteCounts.ReplaceAllString(d, "0 packets, 0 bytes")
		// Indent the result, so that it's treated as preformatted markdown.
		res[k] = strings.ReplaceAll(d, "\n", "\n    ")
	}
	return res
}

func generate(t *testing.T, name string, data map[iptCmdType]string) string {
	t.Helper()
	templ, err := template.New(name).ParseFiles(filepath.Join("templates", name))
	assert.NilError(t, err)
	wr := strings.Builder{}
	err = templ.ExecuteTemplate(&wr, name, data)
	assert.NilError(t, err)
	return wr.String()
}
