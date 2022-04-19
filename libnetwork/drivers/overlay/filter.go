//go:build linux
// +build linux

package overlay

import (
	"fmt"
	"sync"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
	"github.com/sirupsen/logrus"
)

const globalChain = "DOCKER-OVERLAY"

var filterOnce sync.Once

var filterChan = make(chan struct{}, 1)

func filterWait() func() {
	filterChan <- struct{}{}
	return func() { <-filterChan }
}

func getTable() firewallapi.FirewallTable {
	// TODO IPv6 support
	var table firewallapi.FirewallTable
	if err := nftables.InitCheck(); err == nil {
		table = nftables.GetTable(nftables.IPv4)
	} else {
		table = iptables.GetTable(iptables.IPv4)
	}
	return table
}

func setupGlobalChain() {
	// TODO IPv6 support
	table := getTable()

	// Because of an ungraceful shutdown, chain could already be present
	if !table.ExistChain(globalChain, firewallapi.Filter) {
		if _, err := table.NewChain(globalChain, firewallapi.Filter, false); err != nil {
			logrus.Errorf("could not create global overlay chain: %v", err)
			return
		}
	}

	if err := table.AddReturnRule(globalChain); err != nil {
		logrus.Errorf("could not install default return chain in the overlay global chain: %v", err)
	}
}

func setNetworkChain(cname string, remove bool) error {
	// TODO IPv6 support
	table := getTable()

	// Initialize the onetime global overlay chain
	filterOnce.Do(setupGlobalChain)

	exists := table.ExistChain(cname, firewallapi.Filter)

	// In case of remove, make sure to flush the rules in the chain
	if remove && exists {
		if err := table.RemoveExistingChain(cname, firewallapi.Filter); err != nil {
			return fmt.Errorf("failed to flush overlay network chain %s rules: %v", cname, err)
		}
	}

	if !remove && !exists {
		if _, err := table.NewChain(cname, firewallapi.Filter, false); err != nil {
			return fmt.Errorf("failed network chain operation for chain %s: %v", cname, err)
		}
	}

	if !remove {
		if err := table.EnsureDropRule(cname); err != nil {
			return fmt.Errorf("failed adding default drop rule to overlay network chain %s: %v", cname, err)
		}
	}

	return nil
}

func addNetworkChain(cname string) error {
	defer filterWait()()

	return setNetworkChain(cname, false)
}

func removeNetworkChain(cname string) error {
	defer filterWait()()

	return setNetworkChain(cname, true)
}

func setFilters(cname, brName string, remove bool) error {
	// TODO IPv6 support
	table := getTable()

	// Every time we set filters for a new subnet make sure to move the global overlay hook to the top of the both the OUTPUT and forward chains
	if !remove {
		for _, chain := range []string{"OUTPUT", "FORWARD"} {
			if err := table.EnsureJumpRule(chain, globalChain); err != nil {
				return fmt.Errorf("failed to insert overlay hook in chain %s: %v", chain, err)
			}
		}
	}

	// Insert/Delete the rule to jump to per-bridge chain
	if err := table.EnsureJumpRuleForIface(globalChain, cname, brName); err != nil {
		return fmt.Errorf("failed to add per-bridge filter rule for bridge %s, network chain %s: %v", brName, cname, err)
	}

	if err := table.EnsureAcceptRuleForIface(cname, brName); err != nil {
		return fmt.Errorf("failed to add overlay filter rile for network chain %s, bridge %s: %v", cname, brName, err)
	}

	return nil
}

func addFilters(cname, brName string) error {
	defer filterWait()()

	return setFilters(cname, brName, false)
}

func removeFilters(cname, brName string) error {
	defer filterWait()()

	return setFilters(cname, brName, true)
}
