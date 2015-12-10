package overlay

import (
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
)

const globalChain = "DOCKER-OVERLAY"

var filterOnce sync.Once

func rawIPTables(args ...string) error {
	if output, err := iptables.Raw(args...); err != nil {
		return fmt.Errorf("unable to add overlay filter: %v", err)
	} else if len(output) != 0 {
		return fmt.Errorf("unable to add overlay filter: %s", string(output))
	}

	return nil
}

func setupGlobalChain() {
	if err := rawIPTables("-N", globalChain); err != nil {
		logrus.Errorf("could not create global overlay chain: %v", err)
		return
	}

	if err := rawIPTables("-A", globalChain, "-j", "RETURN"); err != nil {
		logrus.Errorf("could not install default return chain in the overlay global chain: %v", err)
		return
	}
}

func setNetworkChain(cname string, remove bool) error {
	// Initialize the onetime global overlay chain
	filterOnce.Do(setupGlobalChain)

	opt := "-N"
	// In case of remove, make sure to flush the rules in the chain
	if remove {
		if err := rawIPTables("-F", cname); err != nil {
			return fmt.Errorf("failed to flush overlay network chain %s rules: %v", cname, err)
		}
		opt = "-X"
	}

	if err := rawIPTables(opt, cname); err != nil {
		return fmt.Errorf("failed network chain operation %q for chain %s: %v", opt, cname, err)
	}

	if !remove {
		if err := rawIPTables("-A", cname, "-j", "DROP"); err != nil {
			return fmt.Errorf("failed adding default drop rule to overlay network chain %s: %v", cname, err)
		}
	}

	return nil
}

func addNetworkChain(cname string) error {
	return setNetworkChain(cname, false)
}

func removeNetworkChain(cname string) error {
	return setNetworkChain(cname, true)
}

func setFilters(cname, brName string, remove bool) error {
	opt := "-I"
	if remove {
		opt = "-D"
	}

	// Everytime we set filters for a new subnet make sure to move the global overlay hook to the top of the both the OUTPUT and forward chains
	if !remove {
		for _, chain := range []string{"OUTPUT", "FORWARD"} {
			exists := iptables.Exists(iptables.Filter, chain, "-j", globalChain)
			if exists {
				if err := rawIPTables("-D", chain, "-j", globalChain); err != nil {
					return fmt.Errorf("failed to delete overlay hook in chain %s while moving the hook: %v", chain, err)
				}
			}

			if err := rawIPTables("-I", chain, "-j", globalChain); err != nil {
				return fmt.Errorf("failed to insert overlay hook in chain %s: %v", chain, err)
			}
		}
	}

	// Insert/Delete the rule to jump to per-bridge chain
	exists := iptables.Exists(iptables.Filter, globalChain, "-o", brName, "-j", cname)
	if (!remove && !exists) || (remove && exists) {
		if err := rawIPTables(opt, globalChain, "-o", brName, "-j", cname); err != nil {
			return fmt.Errorf("failed to add per-bridge filter rule for bridge %s, network chain %s: %v", brName, cname, err)
		}
	}

	exists = iptables.Exists(iptables.Filter, cname, "-i", brName, "-j", "ACCEPT")
	if (!remove && exists) || (remove && !exists) {
		return nil
	}

	if err := rawIPTables(opt, cname, "-i", brName, "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to add overlay filter rile for network chain %s, bridge %s: %v", cname, brName, err)
	}

	return nil
}

func addFilters(cname, brName string) error {
	return setFilters(cname, brName, false)
}

func removeFilters(cname, brName string) error {
	return setFilters(cname, brName, true)
}
