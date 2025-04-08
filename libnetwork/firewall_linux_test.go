package libnetwork

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/libnetwork/drivers/bridge"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
)

const (
	fwdChainName = "FORWARD"
	usrChainName = userChain
)

func TestUserChain(t *testing.T) {
	iptable4 := iptables.GetIptable(iptables.IPv4)
	iptable6 := iptables.GetIptable(iptables.IPv6)

	res := icmd.RunCommand("iptables", "--version")
	assert.NilError(t, res.Error)
	noChainErr := "No chain/target/match by that name"
	if strings.Contains(res.Combined(), "nf_tables") {
		// For a non-existent chain, iptables-nft "-S <chain>" reports:
		//  ip6tables v1.8.9 (nf_tables): chain `<chain>' in table `filter' is incompatible, use 'nft' tool.
		noChainErr = "incompatible, use 'nft' tool"
	}

	tests := []struct {
		iptables bool
		append   bool // append other rules to FORWARD
	}{
		{
			iptables: true,
			append:   false,
		},
		{
			iptables: true,
			append:   true,
		},
		{
			iptables: false,
			append:   false,
		},
	}

	for _, tc := range tests {
		t.Run(fmt.Sprintf("iptables=%v,append=%v", tc.iptables, tc.append), func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			defer resetIptables(t)

			c, err := New(
				context.Background(),
				config.OptionDataDir(t.TempDir()),
				config.OptionDriverConfig("bridge", map[string]any{
					netlabel.GenericData: options.Generic{
						"EnableIPTables":  tc.iptables,
						"EnableIP6Tables": tc.iptables,
					},
				}))
			assert.NilError(t, err)
			defer c.Stop()

			// init. condition
			golden.Assert(t, getRules(t, iptable4, fwdChainName),
				fmt.Sprintf("TestUserChain_iptables-%v_append-%v_fwdinit4", tc.iptables, tc.append))
			golden.Assert(t, getRules(t, iptable6, fwdChainName),
				fmt.Sprintf("TestUserChain_iptables-%v_append-%v_fwdinit6", tc.iptables, tc.append))
			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, bridge.DockerForwardChain),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_dockerfwdinit4", tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, bridge.DockerForwardChain),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_dockerfwdinit6", tc.iptables, tc.append))
			} else {
				assert.Check(t, !iptables.GetIptable(iptables.IPv4).ExistChain(bridge.DockerForwardChain, fwdChainName),
					"Chain %s should not exist", bridge.DockerForwardChain)
			}

			if tc.append {
				_, err := iptable4.Raw("-A", fwdChainName, "-j", "DROP")
				assert.Check(t, err)
				_, err = iptable6.Raw("-A", fwdChainName, "-j", "DROP")
				assert.Check(t, err)
			}
			c.setupUserChains()

			golden.Assert(t, getRules(t, iptable4, fwdChainName),
				fmt.Sprintf("TestUserChain_iptables-%v_append-%v_fwdafter4", tc.iptables, tc.append))
			golden.Assert(t, getRules(t, iptable6, fwdChainName),
				fmt.Sprintf("TestUserChain_iptables-%v_append-%v_fwdafter6", tc.iptables, tc.append))
			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, bridge.DockerForwardChain),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_dockerfwdafter4", tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, bridge.DockerForwardChain),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_dockerfwdafter6", tc.iptables, tc.append))
			} else {
				assert.Check(t, !iptables.GetIptable(iptables.IPv4).ExistChain(bridge.DockerForwardChain, fwdChainName),
					"Chain %s should not exist", bridge.DockerForwardChain)
			}

			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, usrChainName),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_usrafter4", tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, usrChainName),
					fmt.Sprintf("TestUserChain_iptables-%v_append-%v_usrafter6", tc.iptables, tc.append))
			} else {
				_, err := iptable4.Raw("-S", usrChainName)
				assert.Check(t, is.ErrorContains(err, noChainErr), "ipv4 chain %v: created unexpectedly", usrChainName)
				_, err = iptable6.Raw("-S", usrChainName)
				assert.Check(t, is.ErrorContains(err, noChainErr), "ipv6 chain %v: created unexpectedly", usrChainName)
			}
		})
	}
}

func getRules(t *testing.T, iptable *iptables.IPTable, chain string) string {
	t.Helper()
	output, err := iptable.Raw("-S", chain)
	assert.NilError(t, err, "chain %s: failed to get rules", chain)
	return string(output)
}

func resetIptables(t *testing.T) {
	t.Helper()

	for _, ipVer := range []iptables.IPVersion{iptables.IPv4, iptables.IPv6} {
		iptable := iptables.GetIptable(ipVer)

		_, err := iptable.Raw("-F", fwdChainName)
		assert.Check(t, err)
		_ = iptable.RemoveExistingChain(usrChainName, iptables.Filter)
	}
}
