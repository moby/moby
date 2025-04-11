package bridge

import (
	"context"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/types"
	"github.com/vishvananda/netlink"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	defaultBridgeName = "testbridge"
)

func TestProgramIPTable(t *testing.T) {
	// Create a test bridge with a basic bridge configuration (name + IPv4).
	defer netnsutils.SetupTestOSContext(t)()

	_, err := iptables.GetIptable(iptables.IPv4).NewChain(DockerForwardChain, iptables.Filter)
	assert.NilError(t, err)

	// Store various iptables chain rules we care for.
	const iptablesTestBridgeIP = "192.168.42.1"
	rules := []struct {
		rule  iptables.Rule
		descr string
	}{
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-d", "127.1.2.3", "-i", "lo", "-o", "lo", "-j", "DROP"}}, "Test Loopback"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", iptablesTestBridgeIP, "!", "-o", DefaultBridgeName, "-j", "MASQUERADE"}}, "NAT Test"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-o", DefaultBridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}}, "Test ACCEPT INCOMING"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", DefaultBridgeName, "!", "-o", DefaultBridgeName, "-j", "ACCEPT"}}, "Test ACCEPT NON_ICC OUTGOING"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", DefaultBridgeName, "-o", DefaultBridgeName, "-j", "ACCEPT"}}, "Test enable ICC"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", DefaultBridgeName, "-o", DefaultBridgeName, "-j", "DROP"}}, "Test disable ICC"},
	}

	// Assert the chain rules' insertion and removal.
	for _, c := range rules {
		// Add
		if err := programChainRule(c.rule, c.descr, true); err != nil {
			t.Fatalf("Failed to program iptable rule %s: %s", c.descr, err.Error())
		}

		if !c.rule.Exists() {
			t.Fatalf("Failed to effectively program iptable rule: %s", c.descr)
		}

		// Remove
		if err := programChainRule(c.rule, c.descr, false); err != nil {
			t.Fatalf("Failed to remove iptable rule %s: %s", c.descr, err.Error())
		}
		if c.rule.Exists() {
			t.Fatalf("Failed to effectively remove iptable rule: %s", c.descr)
		}
	}
}

func TestSetupIPChains(t *testing.T) {
	// Create a test bridge with a basic bridge configuration (name + IPv4).
	defer netnsutils.SetupTestOSContext(t)()

	ipt := firewaller{IPv4: true}
	err := ipt.init()
	assert.NilError(t, err)

	nc := networkConfig{
		IfName: defaultBridgeName,
		Config4: networkConfigFam{
			Prefix: netip.MustParsePrefix("192.168.42.0/24"),
		},
	}

	assertBridgeConfig(t, ipt, nc)

	nc.Masquerade = true
	assertBridgeConfig(t, ipt, nc)

	nc.ICC = true
	assertBridgeConfig(t, ipt, nc)

	nc.Masquerade = false
	assertBridgeConfig(t, ipt, nc)
}

// Regression test for https://github.com/moby/moby/issues/46445
func TestSetupIP6TablesWithHostIPv4(t *testing.T) {
	defer netnsutils.SetupTestOSContext(t)()
	ipt := firewaller{
		IPv4: true,
		IPv6: true,
	}
	err := ipt.init()
	assert.NilError(t, err)

	nc := networkConfig{
		IfName:     defaultBridgeName,
		Masquerade: true,
		Config4: networkConfigFam{
			HostIP: netip.MustParseAddr("192.0.2.2"),
			Prefix: netip.MustParsePrefix("192.168.42.0/24"),
		},
		Config6: networkConfigFam{
			Prefix: netip.MustParsePrefix("2001:db8::/64"),
		},
	}
	assertBridgeConfig(t, ipt, nc)
}

// Assert function which pushes chains based on bridge config parameters.
func assertBridgeConfig(t *testing.T, ipt firewaller, nc networkConfig) {
	t.Helper()
	n, err := ipt.NewNetwork(nc)
	assert.NilError(t, err)
	err = n.delNetworkLevelRules()
	assert.NilError(t, err)
}

func TestOutgoingNATRules(t *testing.T) {
	br := "br-nattest"
	maskedBrIPv4 := netip.MustParsePrefix("192.168.42.1/16").Masked()
	maskedBrIPv6 := netip.MustParsePrefix("2001:db8::1/64").Masked()
	hostIPv4 := netip.MustParseAddr("192.0.2.2")
	hostIPv6 := netip.MustParseAddr("2001:db8:1::1")
	for _, tc := range []struct {
		desc               string
		enableIPTables     bool
		enableIP6Tables    bool
		enableIPv4         bool
		enableIPv6         bool
		enableIPMasquerade bool
		hostIPv4           netip.Addr
		hostIPv6           netip.Addr
		// Hairpin NAT rules are not tested here because they are orthogonal to outgoing NAT.  They
		// exist to support the port forwarding DNAT rules: without any port forwarding there would be
		// no need for any hairpin NAT rules, and when there is port forwarding then hairpin NAT rules
		// are needed even if outgoing NAT is disabled.  Hairpin NAT tests belong with the port
		// forwarding DNAT tests.
		wantIPv4Masq bool
		wantIPv4Snat bool
		wantIPv6Masq bool
		wantIPv6Snat bool
	}{
		{
			desc:       "everything disabled except ipv4",
			enableIPv4: true, // one of IPv4 or IPv6 must be enabled
		},
		{
			desc:       "everything disabled except ipv6",
			enableIPv6: true,
		},
		{
			desc:               "iptables/ip6tables disabled",
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
		},
		{
			desc:               "host IP with iptables/ip6tables disabled",
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
			hostIPv4:           hostIPv4,
			hostIPv6:           hostIPv6,
		},
		{
			desc:            "masquerade disabled, no host IP",
			enableIPTables:  true,
			enableIP6Tables: true,
			enableIPv4:      true,
			enableIPv6:      true,
		},
		{
			desc:            "masquerade disabled, with host IP",
			enableIPTables:  true,
			enableIP6Tables: true,
			enableIPv4:      true,
			enableIPv6:      true,
			hostIPv4:        hostIPv4,
			hostIPv6:        hostIPv6,
		},
		{
			desc:               "IPv4 masquerade, IPv6 disabled",
			enableIPv4:         true,
			enableIPTables:     true,
			enableIPMasquerade: true,
			wantIPv4Masq:       true,
		},
		{
			desc:               "IPv6 masquerade, IPv4 disabled",
			enableIPv6:         true,
			enableIP6Tables:    true,
			enableIPMasquerade: true,
			wantIPv6Masq:       true,
		},
		{
			desc:               "IPv4 SNAT, IPv6 disabled",
			enableIPv4:         true,
			enableIPTables:     true,
			enableIPMasquerade: true,
			hostIPv4:           hostIPv4,
			wantIPv4Snat:       true,
		},
		{
			desc:               "IPv6 SNAT, IPv4 disabled",
			enableIPv6:         true,
			enableIP6Tables:    true,
			enableIPMasquerade: true,
			hostIPv6:           hostIPv6,
			wantIPv6Snat:       true,
		},
		{
			desc:               "IPv4 masquerade, IPv6 masquerade",
			enableIPTables:     true,
			enableIP6Tables:    true,
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
			wantIPv4Masq:       true,
			wantIPv6Masq:       true,
		},
		{
			desc:               "IPv4 masquerade, IPv6 SNAT",
			enableIPTables:     true,
			enableIP6Tables:    true,
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
			hostIPv6:           hostIPv6,
			wantIPv4Masq:       true,
			wantIPv6Snat:       true,
		},
		{
			desc:               "IPv4 SNAT, IPv6 masquerade",
			enableIPTables:     true,
			enableIP6Tables:    true,
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
			hostIPv4:           hostIPv4,
			wantIPv4Snat:       true,
			wantIPv6Masq:       true,
		},
		{
			desc:               "IPv4 SNAT, IPv6 SNAT",
			enableIPTables:     true,
			enableIP6Tables:    true,
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
			hostIPv4:           hostIPv4,
			hostIPv6:           hostIPv6,
			wantIPv4Snat:       true,
			wantIPv6Snat:       true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()

			ipt := firewaller{
				IPv4: tc.enableIPTables,
				IPv6: tc.enableIP6Tables,
			}
			err := ipt.init()
			assert.NilError(t, err)

			nc := networkConfig{
				IfName:     br,
				Masquerade: tc.enableIPMasquerade,
				Config4: networkConfigFam{
					HostIP: tc.hostIPv4,
					Prefix: maskedBrIPv4,
				},
				Config6: networkConfigFam{
					HostIP: tc.hostIPv6,
					Prefix: maskedBrIPv6,
				},
			}
			n, err := ipt.NewNetwork(nc)
			assert.NilError(t, err)

			defer func() {
				err = n.delNetworkLevelRules()
				assert.NilError(t, err)
			}()

			// Log the contents of all chains to aid troubleshooting.
			for _, ipv := range []iptables.IPVersion{iptables.IPv4, iptables.IPv6} {
				ipt := iptables.GetIptable(ipv)
				for _, table := range []iptables.Table{iptables.Nat, iptables.Filter, iptables.Mangle} {
					out, err := ipt.Raw("-t", string(table), "-S")
					if err != nil {
						t.Error(err)
					}
					t.Logf("%s: %s %s table rules:\n%s", tc.desc, ipv, table, string(out))
				}
			}
			for _, rc := range []struct {
				want bool
				rule iptables.Rule
			}{
				// Rule order doesn't matter: At most one of the following IPv4 rules will exist, and the
				// same goes for the IPv6 rules.
				{tc.wantIPv4Masq, iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", maskedBrIPv4.String(), "!", "-o", br, "-j", "MASQUERADE"}}},
				{tc.wantIPv4Snat, iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", maskedBrIPv4.String(), "!", "-o", br, "-j", "SNAT", "--to-source", hostIPv4.String()}}},
				{tc.wantIPv6Masq, iptables.Rule{IPVer: iptables.IPv6, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", maskedBrIPv6.String(), "!", "-o", br, "-j", "MASQUERADE"}}},
				{tc.wantIPv6Snat, iptables.Rule{IPVer: iptables.IPv6, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", maskedBrIPv6.String(), "!", "-o", br, "-j", "SNAT", "--to-source", hostIPv6.String()}}},
			} {
				assert.Equal(t, rc.rule.Exists(), rc.want)
			}
		})
	}
}

func TestMirroredWSL2Workaround(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		loopback0        bool
		userlandProxy    bool
		wslinfoPerm      os.FileMode // 0 for no-file
		expLoopback0Rule bool
	}{
		{
			desc: "No loopback0",
		},
		{
			desc:             "WSL2 mirrored",
			loopback0:        true,
			userlandProxy:    true,
			wslinfoPerm:      0o777,
			expLoopback0Rule: true,
		},
		{
			desc:          "loopback0 but wslinfo not executable",
			loopback0:     true,
			userlandProxy: true,
			wslinfoPerm:   0o666,
		},
		{
			desc:          "loopback0 but no wslinfo",
			loopback0:     true,
			userlandProxy: true,
		},
		{
			desc:        "loopback0 but no userland proxy",
			loopback0:   true,
			wslinfoPerm: 0o777,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			restoreWslinfoPath := simulateWSL2MirroredMode(t, tc.loopback0, tc.wslinfoPerm)
			defer restoreWslinfoPath()

			config := configuration{EnableIPTables: true}
			if tc.userlandProxy {
				config.UserlandProxyPath = "some-proxy"
				config.EnableUserlandProxy = true
			}
			err := setupIPChains(iptables.IPv4, !tc.userlandProxy)
			assert.NilError(t, err)
			assert.Check(t, is.Equal(mirroredWSL2Rule().Exists(), tc.expLoopback0Rule))
		})
	}
}

// simulateWSL2MirroredMode simulates the WSL2 mirrored mode by creating a
// loopback0 interface and optionally creating a wslinfo file with the given
// permissions.
// A clean up function is returned and will restore the original wslinfoPath
// used within the 'bridge' package. The loopback0 interface isn't cleaned up.
// Instead this function should be called from a disposable network namespace.
func simulateWSL2MirroredMode(t *testing.T, loopback0 bool, wslinfoPerm os.FileMode) func() {
	if loopback0 {
		iface := &netlink.Dummy{
			LinkAttrs: netlink.LinkAttrs{
				Name: "loopback0",
			},
		}
		err := netlink.LinkAdd(iface)
		assert.NilError(t, err)
	}

	wslinfoPathOrig := wslinfoPath
	if wslinfoPerm != 0 {
		tmpdir := t.TempDir()
		p := filepath.Join(tmpdir, "wslinfo")
		err := os.WriteFile(p, []byte("#!/bin/sh\necho dummy file\n"), wslinfoPerm)
		assert.NilError(t, err)
		wslinfoPath = p
	}

	return func() {
		wslinfoPath = wslinfoPathOrig
	}
}

func TestMirroredWSL2LoopbackFiltering(t *testing.T) {
	for _, tc := range []struct {
		desc             string
		loopback0        bool
		wslinfoPerm      os.FileMode // 0 for no-file
		expLoopback0Rule bool
	}{
		{
			desc: "No loopback0",
		},
		{
			desc:             "WSL2 mirrored",
			loopback0:        true,
			wslinfoPerm:      0o777,
			expLoopback0Rule: true,
		},
		{
			desc:        "loopback0 but wslinfo not executable",
			loopback0:   true,
			wslinfoPerm: 0o666,
		},
		{
			desc:      "loopback0 but no wslinfo",
			loopback0: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			restoreWslinfoPath := simulateWSL2MirroredMode(t, tc.loopback0, tc.wslinfoPerm)
			defer restoreWslinfoPath()

			hostIP := net.ParseIP("127.0.0.1")
			err := filterPortMappedOnLoopback(context.TODO(), types.PortBinding{
				Proto:    types.TCP,
				IP:       hostIP,
				HostPort: 8000,
			}, hostIP, true)
			assert.NilError(t, err)

			// Checking this after trying to create rules, to make sure the init code in iptables/firewalld.go has run.
			if fw, _ := iptables.UsingFirewalld(); fw {
				t.Skip("firewalld is running in the host netns, it can't modify rules in the test's netns")
			}

			out, err := exec.Command("iptables-save", "-t", "raw").CombinedOutput()
			assert.NilError(t, err)

			if tc.expLoopback0Rule {
				assert.Check(t, is.Equal(strings.Count(string(out), "-A PREROUTING"), 2))
				assert.Check(t, is.Contains(string(out), "-A PREROUTING -d 127.0.0.1/32 -i loopback0 -p tcp -m tcp --dport 8000 -j ACCEPT"))
			} else {
				assert.Check(t, is.Equal(strings.Count(string(out), "-A PREROUTING"), 1))
				assert.Check(t, !strings.Contains(string(out), "loopback0"), "There should be no rule in the raw-PREROUTING chain")
			}
		})
	}
}
