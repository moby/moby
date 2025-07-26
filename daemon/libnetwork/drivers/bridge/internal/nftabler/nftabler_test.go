//go:build linux

package nftabler

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

func TestNftabler(t *testing.T) {
	const (
		ipv4 uint64 = 1 << iota
		ipv6
		hairpin
		internal
		icc
		masq
		snat
		bindLocalhost
		wsl2Mirrored
		numBoolParams = iota
	)
	nftables.Enable()
	t.Cleanup(func() { nftables.Disable() }) // Cleanup instead of defer, this func returns before the parallel subtests finish.
	for i := range uint64(1) << numBoolParams {
		p := func(n uint64) bool { return (i & n) == n }
		for _, gwmode := range []string{"nat", "nat-unprotected", "routed"} {
			config := firewaller.Config{
				IPv4:         p(ipv4),
				IPv6:         p(ipv6),
				Hairpin:      p(hairpin),
				WSL2Mirrored: p(wsl2Mirrored),
			}
			netConfig := firewaller.NetworkConfig{
				IfName:     "br-dummy",
				Internal:   p(internal),
				ICC:        p(icc),
				Masquerade: p(masq),
				Config4: firewaller.NetworkConfigFam{
					HostIP:      netip.Addr{},
					Prefix:      netip.MustParsePrefix("192.168.0.0/24"),
					Routed:      gwmode == "routed",
					Unprotected: gwmode == "nat-unprotected",
				},
				Config6: firewaller.NetworkConfigFam{
					HostIP:      netip.Addr{},
					Prefix:      netip.MustParsePrefix("fd49:efd7:54aa::/64"),
					Routed:      gwmode == "routed",
					Unprotected: gwmode == "nat-unprotected",
				},
			}
			if p(snat) {
				netConfig.Config4.HostIP = netip.MustParseAddr("192.168.123.0")
				netConfig.Config6.HostIP = netip.MustParseAddr("fd34:d0d4:672f::123")
			}
			tn := t.Name()
			t.Run(fmt.Sprintf("ipv4=%v/ipv6=%v/hairpin=%v/internal=%v/icc=%v/masq=%v/snat=%v/gwm=%v/bindlh=%v/wsl2mirrored=%v",
				p(ipv4), p(ipv6), p(hairpin), p(internal), p(icc), p(masq), p(snat), gwmode, p(bindLocalhost), p(wsl2Mirrored)), func(t *testing.T) {
				// If updating results, don't run in parallel because some of the results files are shared.
				if !golden.FlagUpdate() {
					t.Parallel()
				}
				// Combine results (golden output files) where possible to:
				// - check params that should have no effect when made irrelevant by other params, and
				// - minimise the number of results files.
				var resName string
				if p(internal) {
					// Port binding params should have no effect on an internal network.
					resName = fmt.Sprintf("hairpin=%v,internal=true,icc=%v", p(hairpin), p(icc))
				} else {
					resName = fmt.Sprintf("hairpin=%v,internal=%v,icc=%v,masq=%v,snat=%v,gwm=%v,bindlh=%v",
						p(hairpin), p(internal), p(icc), p(masq), p(snat), gwmode, p(bindLocalhost))
				}
				testNftabler(t, tn, config, netConfig, p(bindLocalhost), tn+"_"+resName)
			})
		}
	}
}

func testNftabler(t *testing.T, tn string, config firewaller.Config, netConfig firewaller.NetworkConfig, bindLocalhost bool, resName string) {
	defer netnsutils.SetupTestOSContext(t)()

	checkResults := func(family, name string, en bool) {
		t.Helper()
		res := icmd.RunCommand("nft", "list", "table", family, dockerTable)
		if !en {
			assert.Assert(t, is.Contains(res.Combined(), "No such file or directory"))
			return
		}
		assert.Assert(t, res.Error)
		golden.Assert(t, res.Combined(), name+"__"+family+".golden")
	}

	makePB := func(hip string, cip netip.Addr) types.PortBinding {
		return types.PortBinding{
			Proto:       types.TCP,
			IP:          cip.AsSlice(),
			Port:        80,
			HostIP:      net.ParseIP(hip),
			HostPort:    8080,
			HostPortEnd: 8080,
		}
	}

	// WSL2Mirrored should only affect IPv4 results, and only if there's a port binding
	// to a loopback address or docker-proxy is disabled. Share other results files.
	rnWSL2Mirrored := func(resName string) string {
		if config.IPv4 && config.WSL2Mirrored && (bindLocalhost || !config.Hairpin) {
			return resName + ",wsl2mirrored=true"
		}
		return resName
	}

	// Initialise iptables, check the iptables config looks like it should look at the
	// end of the test (after deleting per-network and per-port rules).
	fw, err := NewNftabler(context.Background(), config)
	assert.NilError(t, err)
	checkResults("ip", rnWSL2Mirrored(fmt.Sprintf("%s_cleaned,hairpin=%v", tn, config.Hairpin)), config.IPv4)
	checkResults("ip6", fmt.Sprintf("%s_cleaned,hairpin=%v", tn, config.Hairpin), config.IPv6)

	// Add the network.
	nw, err := fw.NewNetwork(context.Background(), netConfig)
	assert.NilError(t, err)

	// Add an endpoint.
	epAddr4 := netip.MustParseAddr("192.168.0.2")
	epAddr6 := netip.MustParseAddr("fd49:efd7:54aa::1")
	err = nw.AddEndpoint(context.Background(), epAddr4, epAddr6)
	assert.NilError(t, err)

	// Add IPv4 and IPv6 port mappings.
	var pb4, pb6 types.PortBinding
	if bindLocalhost {
		pb4 = makePB("127.0.0.1", epAddr4)
		pb6 = makePB("::1", epAddr6)
	} else {
		pb4 = makePB("0.0.0.0", epAddr4)
		pb6 = makePB("::", epAddr6)
	}
	err = nw.AddPorts(context.Background(), []types.PortBinding{pb4, pb6})
	assert.NilError(t, err)

	// Check the resulting iptables config.
	checkResults("ip", rnWSL2Mirrored(resName), config.IPv4)
	checkResults("ip6", resName, config.IPv6)

	// Remove the port mappings and the network, and check the result (should be the same
	// for all tests with the same "hairpin" setting).
	err = nw.DelPorts(context.Background(), []types.PortBinding{pb4, pb6})
	assert.NilError(t, err)
	err = nw.DelEndpoint(context.Background(), epAddr4, epAddr6)
	assert.NilError(t, err)
	err = nw.DelNetworkLevelRules(context.Background())
	assert.NilError(t, err)
	checkResults("ip", rnWSL2Mirrored(fmt.Sprintf("%s_cleaned,hairpin=%v", tn, config.Hairpin)), config.IPv4)
	checkResults("ip6", fmt.Sprintf("%s_cleaned,hairpin=%v", tn, config.Hairpin), config.IPv6)
}
