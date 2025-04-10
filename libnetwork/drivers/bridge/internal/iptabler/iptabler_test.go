//go:build linux

package iptabler

import (
	"testing"

	"github.com/docker/docker/internal/testutils/netnsutils"
	"github.com/docker/docker/libnetwork/iptables"
	"gotest.tools/v3/assert"
)

func TestCleanupIptableRules(t *testing.T) {
	// Check for existence of a dummy rule to make sure iptables is initialised - then can
	// check whether firewalld is running.
	_ = iptables.GetIptable(iptables.IPv4).Exists(iptables.Filter, "FORWARD", "-j", "DROP")
	if fw, _ := iptables.UsingFirewalld(); fw {
		t.Skip("firewalld is running in the host netns, it can't modify rules in the test's netns")
	}

	defer netnsutils.SetupTestOSContext(t)()
	bridgeChains := []struct {
		name       string
		table      iptables.Table
		expRemoved bool
	}{
		{name: dockerChain, table: iptables.Nat, expRemoved: true},
		// The filter-FORWARD chain has references to dockerChain and isolationChain1,
		// so the chains won't be removed - but they should be flushed. (This has
		// long/always been the case for the daemon, its filter-FORWARD rules aren't
		// removed.)
		{name: dockerChain, table: iptables.Filter},
		{name: isolationChain1, table: iptables.Filter},
	}

	ipVersions := []iptables.IPVersion{iptables.IPv4, iptables.IPv6}

	for _, version := range ipVersions {
		err := setupIPChains(version, true)
		assert.NilError(t, err, "version:%s", version)

		iptable := iptables.GetIptable(version)
		for _, chainInfo := range bridgeChains {
			exists := iptable.ExistChain(chainInfo.name, chainInfo.table)
			assert.Check(t, exists, "version:%s chain:%s table:%v",
				version, chainInfo.name, chainInfo.table)
		}

		// Insert RETURN rules so that there's something to flush.
		for _, chainInfo := range bridgeChains {
			out, err := iptable.Raw("-t", string(chainInfo.table), "-A", chainInfo.name, "-j", "RETURN")
			assert.NilError(t, err, "version:%s chain:%s table:%v out:%s",
				version, chainInfo.name, chainInfo.table, out)
		}

		removeIPChains(version)

		for _, chainInfo := range bridgeChains {
			exists := iptable.Exists(chainInfo.table, chainInfo.name, "-A", chainInfo.name, "-j", "RETURN")
			assert.Check(t, !exists, "version:%s chain:%s table:%v",
				version, chainInfo.name, chainInfo.table)
			if chainInfo.expRemoved {
				exists := iptable.ExistChain(chainInfo.name, chainInfo.table)
				assert.Check(t, !exists, "version:%s chain:%s table:%v",
					version, chainInfo.name, chainInfo.table)
			}
		}
	}
}
