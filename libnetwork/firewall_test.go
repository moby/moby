package libnetwork

import (
	"fmt"
	"gotest.tools/assert"
	"strings"
	"testing"

	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netlabel"
	"github.com/docker/libnetwork/options"
)

const (
	fwdChainName = "FORWARD"
	usrChainName = userChain
)

func TestUserChain(t *testing.T) {
	nc, err := New()
	assert.NilError(t, err)

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

	resetIptables(t)
	for _, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("iptables=%v,insert=%v", tc.iptables, tc.insert), func(t *testing.T) {
			c := nc.(*controller)
			c.cfg.Daemon.DriverCfg["bridge"] = map[string]interface{}{
				netlabel.GenericData: options.Generic{
					"EnableIPTables": tc.iptables,
				},
			}

			// init. condition, FORWARD chain empty DOCKER-USER not exist
			assert.DeepEqual(t, getRules(t, fwdChainName), []string{"-P FORWARD ACCEPT"})

			if tc.insert {
				_, err = iptables.Raw("-A", fwdChainName, "-j", "DROP")
				assert.NilError(t, err)
			}
			arrangeUserFilterRule()

			assert.DeepEqual(t, getRules(t, fwdChainName), tc.fwdChain)
			if tc.userChain != nil {
				assert.DeepEqual(t, getRules(t, usrChainName), tc.userChain)
			} else {
				_, err := iptables.Raw("-S", usrChainName)
				assert.Assert(t, err != nil, "chain %v: created unexpectedly", usrChainName)
			}
		})
		resetIptables(t)
	}
}

func getRules(t *testing.T, chain string) []string {
	t.Helper()
	output, err := iptables.Raw("-S", chain)
	assert.NilError(t, err, "chain %s: failed to get rules", chain)

	rules := strings.Split(string(output), "\n")
	if len(rules) > 0 {
		rules = rules[:len(rules)-1]
	}
	return rules
}

func resetIptables(t *testing.T) {
	t.Helper()
	_, err := iptables.Raw("-F", fwdChainName)
	assert.NilError(t, err)
	_ = iptables.RemoveExistingChain(usrChainName, "")
}
