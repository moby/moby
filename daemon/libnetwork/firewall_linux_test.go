package libnetwork

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
	"github.com/moby/moby/v2/internal/testutil/netnsutils"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
	"gotest.tools/v3/golden"
	"gotest.tools/v3/icmd"
	"gotest.tools/v3/skip"
)

const (
	fwdChainName = "FORWARD"
	usrChainName = userChain
)

func TestUserChain(t *testing.T) {
	const testName = "TestUserChain"
	iptable4 := iptables.GetIptable(iptables.IPv4)
	iptable6 := iptables.GetIptable(iptables.IPv6)

	res := icmd.RunCommand("iptables", "--version")
	assert.NilError(t, res.Error)
	noChainErr := "No chain/target/match by that name"
	iptVerLt := versionLt(t, res.Combined(), 1, 8, 10)
	t.Logf("iptables version < v1.8.11: %t", iptVerLt)
	if strings.Contains(res.Combined(), "nf_tables") && iptVerLt {
		// Prior to v1.8.11, iptables-nft "-S <chain>" reports the following for a non-existent chain:
		//
		//   ip6tables v1.8.9 (nf_tables): chain `<chain>' in table `filter' is incompatible, use 'nft' tool.
		//
		// This was fixed in this commit: https://git.netfilter.org/iptables/commit/?id=82ccfb488eeac5507471099b9b4e6d136cc06e3b
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
				config.OptionBridgeConfig(bridge.Configuration{
					EnableIPTables:  tc.iptables,
					EnableIP6Tables: tc.iptables,
				}))
			assert.NilError(t, err)
			defer c.Stop()
			skip.If(t, nftables.Enabled(), "nftables is enabled, skipping iptables test")

			// init. condition
			golden.Assert(t, getRules(t, iptable4, fwdChainName),
				fmt.Sprintf("%s/iptables-%v_append-%v_fwdinit4.golden", testName, tc.iptables, tc.append))
			golden.Assert(t, getRules(t, iptable6, fwdChainName),
				fmt.Sprintf("%s/iptables-%v_append-%v_fwdinit6.golden", testName, tc.iptables, tc.append))
			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, bridge.DockerForwardChain),
					fmt.Sprintf("%s/iptables-%v_append-%v_dockerfwdinit4.golden", testName, tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, bridge.DockerForwardChain),
					fmt.Sprintf("%s/iptables-%v_append-%v_dockerfwdinit6.golden", testName, tc.iptables, tc.append))
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
				fmt.Sprintf("%s/iptables-%v_append-%v_fwdafter4.golden", testName, tc.iptables, tc.append))
			golden.Assert(t, getRules(t, iptable6, fwdChainName),
				fmt.Sprintf("%s/iptables-%v_append-%v_fwdafter6.golden", testName, tc.iptables, tc.append))
			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, bridge.DockerForwardChain),
					fmt.Sprintf("%s/iptables-%v_append-%v_dockerfwdafter4.golden", testName, tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, bridge.DockerForwardChain),
					fmt.Sprintf("%s/iptables-%v_append-%v_dockerfwdafter6.golden", testName, tc.iptables, tc.append))
			} else {
				assert.Check(t, !iptables.GetIptable(iptables.IPv4).ExistChain(bridge.DockerForwardChain, fwdChainName),
					"Chain %s should not exist", bridge.DockerForwardChain)
			}

			if tc.iptables {
				golden.Assert(t, getRules(t, iptable4, usrChainName),
					fmt.Sprintf("%s/iptables-%v_append-%v_usrafter4.golden", testName, tc.iptables, tc.append))
				golden.Assert(t, getRules(t, iptable6, usrChainName),
					fmt.Sprintf("%s/iptables-%v_append-%v_usrafter6.golden", testName, tc.iptables, tc.append))
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

// versionLt returns true if the iptables version returned by `iptables --version`
// is less than the `<major>.<minor>.<patch>` version passed in as argument.
func versionLt(t *testing.T, ver string, major, minor, patch int) bool {
	t.Helper()

	matches := regexp.MustCompile(`iptables v([0-9]+)\.([0-9]+)\.([0-9]+)`).FindStringSubmatch(ver)
	assert.Assert(t, len(matches) == 4, "could not determine iptables version from %q", ver)

	parsedMajor, err := strconv.Atoi(matches[1])
	assert.NilError(t, err)
	parsedMinor, err := strconv.Atoi(matches[2])
	assert.NilError(t, err)
	parsedPatch, err := strconv.Atoi(matches[3])
	assert.NilError(t, err)

	return parsedMajor <= major && parsedMinor <= minor && parsedPatch < patch
}
