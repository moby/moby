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
	"strings"
	"testing"
	"text/template"

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
	"gotest.tools/v3/golden"
	"gotest.tools/v3/skip"
)

var (
	docNetworks = []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"}
	docGateways = []string{"192.0.2.1", "198.51.100.1", "203.0.113.1"}
)

type ctr struct {
	name         string
	portMappings nat.PortMap
}

type bridgeNetwork struct {
	bridge     string
	gwMode     string
	noICC      bool
	internal   bool
	containers []ctr
}

type section struct {
	name            string
	noUserlandProxy bool
	networks        []bridgeNetwork
}

var index = []section{
	{
		name: "new-daemon.md",
	},
	{
		name: "usernet-portmap.md",
		networks: []bridgeNetwork{{
			bridge: "bridge1",
			containers: []ctr{
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
		networks: []bridgeNetwork{{
			bridge: "bridge1",
			containers: []ctr{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name: "usernet-portmap-noicc.md",
		networks: []bridgeNetwork{{
			bridge: "bridge1",
			noICC:  true,
			containers: []ctr{
				{
					name:         "c1",
					portMappings: nat.PortMap{"80/tcp": {{HostPort: "8080"}}},
				},
			},
		}},
	},
	{
		name: "usernet-internal.md",
		networks: []bridgeNetwork{{
			bridge:   "bridge1",
			internal: true,
			containers: []ctr{
				{
					name: "c1",
				},
			},
		}},
	},
	{
		name: "usernet-portmap-routed.md",
		networks: []bridgeNetwork{{
			bridge: "bridge1",
			gwMode: "routed",
			containers: []ctr{
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
	iptCmdSFilterForward4 iptCmdType = "SFilterForward4"
	iptCmdSFilterDocker4  iptCmdType = "SFilterDocker4"
	iptCmdLNat4           iptCmdType = "LNat4"
	iptCmdSNat4           iptCmdType = "SNat4"
)

var iptCmds = map[iptCmdType][]string{
	iptCmdLFilter4:        {"iptables", "-nvL", "--line-numbers", "-t", "filter"},
	iptCmdSFilter4:        {"iptables", "-S", "-t", "filter"},
	iptCmdSFilterForward4: {"iptables", "-S", "FORWARD"},
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
		host.Run(t, "ip", "link", "set", "eth0", "down")

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

	// Start the daemon in its own network namespace.
	var d *daemon.Daemon
	host.Do(t, func() {
		// Run without OTEL because there's no routing from this netns for it - which
		// means the daemon doesn't shut down cleanly, causing the test to fail.
		d = daemon.New(t, daemon.WithEnvVars("OTEL_EXPORTER_OTLP_ENDPOINT="))
		d.StartWithBusybox(ctx, t, dArgs...)
		t.Cleanup(func() { d.Stop(t) })
	})

	c := d.NewClientT(t)
	t.Cleanup(func() { c.Close() })

	assert.Assert(t, len(section.networks) < len(docNetworks), "Don't have enough container network addresses")
	for i, nw := range section.networks {
		gwMode := nw.gwMode
		if gwMode == "" {
			gwMode = "nat"
		}
		netOpts := []func(*networktypes.CreateOptions){
			network.WithIPAM(docNetworks[i], docGateways[i]),
			network.WithOption(bridge.BridgeName, nw.bridge),
			network.WithOption(bridge.IPv4GatewayMode, gwMode),
		}
		if nw.noICC {
			netOpts = append(netOpts, network.WithOption(bridge.EnableICC, "false"))
		}
		if nw.internal {
			netOpts = append(netOpts, network.WithInternal())
		}
		network.CreateNoError(ctx, t, c, nw.bridge, netOpts...)
		t.Cleanup(func() { network.RemoveNoError(ctx, t, c, nw.bridge) })

		for _, ctr := range nw.containers {
			var exposedPorts []string
			for ep := range ctr.portMappings {
				exposedPorts = append(exposedPorts, ep.Port()+"/"+ep.Proto())
			}
			id := container.Run(ctx, t, c,
				container.WithNetworkMode(nw.bridge),
				container.WithExposedPorts(exposedPorts...),
				container.WithPortMap(ctr.portMappings),
			)
			t.Cleanup(func() {
				c.ContainerRemove(ctx, id, containertypes.RemoveOptions{Force: true})
			})
		}
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

var rePacketByteCounts = regexp.MustCompile(`\d+ packets, \d+ bytes`)

func runIptables(t *testing.T, host networking.Host) map[iptCmdType]string {
	host.Run(t, "iptables", "-Z")
	host.Run(t, "iptables", "-Z", "-t", "nat")
	res := map[iptCmdType]string{}
	for k, cmd := range iptCmds {
		d := host.Run(t, cmd[0], cmd[1:]...)
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
