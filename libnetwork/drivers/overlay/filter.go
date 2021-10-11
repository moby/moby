//go:build linux
// +build linux

package overlay

import (
	"fmt"
	"sync"

	"github.com/docker/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
)

const globalChain = "DOCKER-OVERLAY"

var filterOnce sync.Once

var filterChan = make(chan struct{}, 1)

func filterWait() func() {
	filterChan <- struct{}{}
	return func() { <-filterChan }
}

func chainExists(cname string) bool {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	if _, err := iptable.Raw("-L", cname); err != nil {
		return false
	}

	return true
}

func setupGlobalChain() {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	// Because of an ungraceful shutdown, chain could already be present
	if !chainExists(globalChain) {
		if err := iptable.RawCombinedOutput("-N", globalChain); err != nil {
			logrus.Errorf("could not create global overlay chain: %v", err)
			return
		}
	}

	if !iptable.Exists(iptables.Filter, globalChain, "-j", "RETURN") {
		if err := iptable.RawCombinedOutput("-A", globalChain, "-j", "RETURN"); err != nil {
			logrus.Errorf("could not install default return chain in the overlay global chain: %v", err)
		}
	}
}

func setNetworkChain(cname string, remove bool) error {
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)
	// Initialize the onetime global overlay chain
	filterOnce.Do(setupGlobalChain)

	exists := chainExists(cname)

	opt := "-N"
	// In case of remove, make sure to flush the rules in the chain
	if remove && exists {
		if err := iptable.RawCombinedOutput("-F", cname); err != nil {
			return fmt.Errorf("failed to flush overlay network chain %s rules: %v", cname, err)
		}
		opt = "-X"
	}

	if (!remove && !exists) || (remove && exists) {
		if err := iptable.RawCombinedOutput(opt, cname); err != nil {
			return fmt.Errorf("failed network chain operation %q for chain %s: %v", opt, cname, err)
		}
	}

	if !remove {
		if !iptable.Exists(iptables.Filter, cname, "-j", "DROP") {
			if err := iptable.RawCombinedOutput("-A", cname, "-j", "DROP"); err != nil {
				return fmt.Errorf("failed adding default drop rule to overlay network chain %s: %v", cname, err)
			}
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
	opt := "-I"
	if remove {
		opt = "-D"
	}
	// TODO IPv6 support
	iptable := iptables.GetIptable(iptables.IPv4)

	// Every time we set filters for a new subnet make sure to move the global overlay hook to the top of the both the OUTPUT and forward chains
	if !remove {
		for _, chain := range []string{"OUTPUT", "FORWARD"} {
			exists := iptable.Exists(iptables.Filter, chain, "-j", globalChain)
			if exists {
				if err := iptable.RawCombinedOutput("-D", chain, "-j", globalChain); err != nil {
					return fmt.Errorf("failed to delete overlay hook in chain %s while moving the hook: %v", chain, err)
				}
			}

			if err := iptable.RawCombinedOutput("-I", chain, "-j", globalChain); err != nil {
				return fmt.Errorf("failed to insert overlay hook in chain %s: %v", chain, err)
			}
		}
	}

	// Insert/Delete the rule to jump to per-bridge chain
	exists := iptable.Exists(iptables.Filter, globalChain, "-o", brName, "-j", cname)
	if (!remove && !exists) || (remove && exists) {
		if err := iptable.RawCombinedOutput(opt, globalChain, "-o", brName, "-j", cname); err != nil {
			return fmt.Errorf("failed to add per-bridge filter rule for bridge %s, network chain %s: %v", brName, cname, err)
		}
	}

	exists = iptable.Exists(iptables.Filter, cname, "-i", brName, "-j", "ACCEPT")
	if (!remove && exists) || (remove && !exists) {
		return nil
	}

	if err := iptable.RawCombinedOutput(opt, cname, "-i", brName, "-j", "ACCEPT"); err != nil {
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
