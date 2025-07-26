//go:build linux

package iptabler

import (
	"context"
	"net/netip"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
	"github.com/moby/moby/v2/internal/testutils/netnsutils"
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
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Nat, Chain: "POSTROUTING", Args: []string{"-s", iptablesTestBridgeIP, "!", "-o", defaultBridgeName, "-j", "MASQUERADE"}}, "NAT Test"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-o", defaultBridgeName, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}}, "Test ACCEPT INCOMING"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", defaultBridgeName, "!", "-o", defaultBridgeName, "-j", "ACCEPT"}}, "Test ACCEPT NON_ICC OUTGOING"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", defaultBridgeName, "-o", defaultBridgeName, "-j", "ACCEPT"}}, "Test enable ICC"},
		{iptables.Rule{IPVer: iptables.IPv4, Table: iptables.Filter, Chain: DockerForwardChain, Args: []string{"-i", defaultBridgeName, "-o", defaultBridgeName, "-j", "DROP"}}, "Test disable ICC"},
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

	ipt, err := NewIptabler(context.Background(), firewaller.Config{IPv4: true})
	assert.NilError(t, err)

	nc := firewaller.NetworkConfig{
		IfName: defaultBridgeName,
		Config4: firewaller.NetworkConfigFam{
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

	ipt, err := NewIptabler(context.Background(), firewaller.Config{
		IPv4: true,
		IPv6: true,
	})
	assert.NilError(t, err)

	nc := firewaller.NetworkConfig{
		IfName:     defaultBridgeName,
		Masquerade: true,
		Config4: firewaller.NetworkConfigFam{
			HostIP: netip.MustParseAddr("192.0.2.2"),
			Prefix: netip.MustParsePrefix("192.168.42.0/24"),
		},
		Config6: firewaller.NetworkConfigFam{
			Prefix: netip.MustParsePrefix("2001:db8::/64"),
		},
	}
	assertBridgeConfig(t, ipt, nc)
}

// Assert function which pushes chains based on bridge config parameters.
func assertBridgeConfig(t *testing.T, ipt firewaller.Firewaller, nc firewaller.NetworkConfig) {
	t.Helper()
	n, err := ipt.NewNetwork(context.Background(), nc)
	assert.NilError(t, err)
	err = n.DelNetworkLevelRules(context.Background())
	assert.NilError(t, err)
}

func TestOutgoingNATRules(t *testing.T) {
	const br = "br-nattest"
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
			desc:               "iptables and ip6tables disabled",
			enableIPv4:         true,
			enableIPv6:         true,
			enableIPMasquerade: true,
		},
		{
			desc:               "host IP with iptables and ip6tables disabled",
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
			ipt, err := NewIptabler(context.Background(), firewaller.Config{
				IPv4: tc.enableIPTables,
				IPv6: tc.enableIP6Tables,
			})
			assert.NilError(t, err)

			nc := firewaller.NetworkConfig{
				IfName:     br,
				Masquerade: tc.enableIPMasquerade,
				Config4: firewaller.NetworkConfigFam{
					HostIP: tc.hostIPv4,
					Prefix: maskedBrIPv4,
				},
				Config6: firewaller.NetworkConfigFam{
					HostIP: tc.hostIPv6,
					Prefix: maskedBrIPv6,
				},
			}
			n, err := ipt.NewNetwork(context.Background(), nc)
			assert.NilError(t, err)

			defer func() {
				err = n.DelNetworkLevelRules(context.Background())
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
			for i, rc := range []struct {
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
				assert.Check(t, is.Equal(rc.rule.Exists(), rc.want), "rule:%d", i)
			}
		})
	}
}
