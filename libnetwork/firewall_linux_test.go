package libnetwork

import (
	"fmt"
	"strings"
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/config"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/netlabel"
	"github.com/docker/docker/libnetwork/options"
	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

const (
	fwdChainName = "FORWARD"
	usrChainName = userChain
)

func TestUserChain(t *testing.T) {
	iptable4 := iptables.GetIptable(iptables.IPv4)
	iptable6 := iptables.GetIptable(iptables.IPv6)

	tests := []struct {
		iptables  bool
		insert    bool // insert other rules to FORWARD
		fwdChain  []string
		userChain []string
	}{
		{
			iptables: false,
			insert:   false,
			fwdChain: []string{"-P FORWARD ACCEPT"},
		},
		{
			iptables:  true,
			insert:    false,
			fwdChain:  []string{"-P FORWARD ACCEPT", "-A FORWARD -j DOCKER-USER"},
			userChain: []string{"-N DOCKER-USER", "-A DOCKER-USER -j RETURN"},
		},
		{
			iptables:  true,
			insert:    true,
			fwdChain:  []string{"-P FORWARD ACCEPT", "-A FORWARD -j DOCKER-USER", "-A FORWARD -j DROP"},
			userChain: []string{"-N DOCKER-USER", "-A DOCKER-USER -j RETURN"},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("iptables=%v,insert=%v", tc.iptables, tc.insert), func(t *testing.T) {
			defer netnsutils.SetupTestOSContext(t)()
			defer resetIptables(t)

			c, err := New(
				OptionBoltdbWithRandomDBFile(t),
				config.OptionDriverConfig("bridge", map[string]any{
					netlabel.GenericData: options.Generic{
						"EnableIPTables":  tc.iptables,
						"EnableIP6Tables": tc.iptables,
					},
				}))
			assert.NilError(t, err)
			defer c.Stop()

			// init. condition, FORWARD chain empty DOCKER-USER not exist
			assert.Check(t, is.DeepEqual(getRules(t, iptable4, fwdChainName), []string{"-P FORWARD ACCEPT"}))
			assert.Check(t, is.DeepEqual(getRules(t, iptable6, fwdChainName), []string{"-P FORWARD ACCEPT"}))

			if tc.insert {
				_, err = iptable4.Raw("-A", fwdChainName, "-j", "DROP")
				assert.Check(t, err)
				_, err = iptable6.Raw("-A", fwdChainName, "-j", "DROP")
				assert.Check(t, err)
			}
			arrangeUserFilterRule()

			assert.Check(t, is.DeepEqual(getRules(t, iptable4, fwdChainName), tc.fwdChain))
			assert.Check(t, is.DeepEqual(getRules(t, iptable6, fwdChainName), tc.fwdChain))
			if tc.userChain != nil {
				assert.Check(t, is.DeepEqual(getRules(t, iptable4, usrChainName), tc.userChain))
				assert.Check(t, is.DeepEqual(getRules(t, iptable6, usrChainName), tc.userChain))
			} else {
				_, err = iptable4.Raw("-S", usrChainName)
				assert.Check(t, is.ErrorContains(err, "No chain/target/match by that name"), "ipv4 chain %v: created unexpectedly", usrChainName)
				_, err = iptable6.Raw("-S", usrChainName)
				assert.Check(t, is.ErrorContains(err, "No chain/target/match by that name"), "ipv6 chain %v: created unexpectedly", usrChainName)
			}
		})
	}
}

func getRules(t *testing.T, iptable *iptables.IPTable, chain string) []string {
	t.Helper()
	output, err := iptable.Raw("-S", chain)
	assert.NilError(t, err, "chain %s: failed to get rules", chain)

	rules := strings.Split(string(output), "\n")
	if len(rules) > 0 {
		rules = rules[:len(rules)-1]
	}
	return rules
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
