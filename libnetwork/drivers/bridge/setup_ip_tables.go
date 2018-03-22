package bridge

import (
	"errors"
	"fmt"
	"net"

	"github.com/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

// DockerChain: DOCKER iptable chain name
const (
	DockerChain = "DOCKER"
	// Isolation between bridge networks is achieved in two stages by means
	// of the following two chains in the filter table. The first chain matches
	// on the source interface being a bridge network's bridge and the
	// destination being a different interface. A positive match leads to the
	// second isolation chain. No match returns to the parent chain. The second
	// isolation chain matches on destination interface being a bridge network's
	// bridge. A positive match identifies a packet originated from one bridge
	// network's bridge destined to another bridge network's bridge and will
	// result in the packet being dropped. No match returns to the parent chain.
	IsolationChain1 = "DOCKER-ISOLATION-STAGE-1"
	IsolationChain2 = "DOCKER-ISOLATION-STAGE-2"
)

func setupIPChains(config *configuration) (*iptables.ChainInfo, *iptables.ChainInfo, *iptables.ChainInfo, *iptables.ChainInfo, error) {
	// Sanity check.
	if config.EnableIPTables == false {
		return nil, nil, nil, nil, errors.New("cannot create new chains, EnableIPTable is disabled")
	}

	hairpinMode := !config.EnableUserlandProxy

	natChain, err := iptables.NewChain(DockerChain, iptables.Nat, hairpinMode)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create NAT chain %s: %v", DockerChain, err)
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(DockerChain, iptables.Nat); err != nil {
				logrus.Warnf("failed on removing iptables NAT chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	filterChain, err := iptables.NewChain(DockerChain, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER chain %s: %v", DockerChain, err)
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(DockerChain, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	isolationChain1, err := iptables.NewChain(IsolationChain1, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(IsolationChain1, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain1, err)
			}
		}
	}()

	isolationChain2, err := iptables.NewChain(IsolationChain2, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(IsolationChain2, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain2, err)
			}
		}
	}()

	if err := iptables.AddReturnRule(IsolationChain1); err != nil {
		return nil, nil, nil, nil, err
	}

	if err := iptables.AddReturnRule(IsolationChain2); err != nil {
		return nil, nil, nil, nil, err
	}

	return natChain, filterChain, isolationChain1, isolationChain2, nil
}

func (n *bridgeNetwork) setupIPTables(config *networkConfiguration, i *bridgeInterface) error {
	var err error

	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if driverConfig.EnableIPTables == false {
		return errors.New("Cannot program chains, EnableIPTable is disabled")
	}

	// Pickup this configuration option from driver
	hairpinMode := !driverConfig.EnableUserlandProxy

	maskedAddrv4 := &net.IPNet{
		IP:   i.bridgeIPv4.IP.Mask(i.bridgeIPv4.Mask),
		Mask: i.bridgeIPv4.Mask,
	}
	if config.Internal {
		if err = setupInternalNetworkRules(config.BridgeName, maskedAddrv4, config.EnableICC, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupInternalNetworkRules(config.BridgeName, maskedAddrv4, config.EnableICC, false)
		})
	} else {
		if err = setupIPTablesInternal(config.BridgeName, maskedAddrv4, config.EnableICC, config.EnableIPMasquerade, hairpinMode, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupIPTablesInternal(config.BridgeName, maskedAddrv4, config.EnableICC, config.EnableIPMasquerade, hairpinMode, false)
		})
		natChain, filterChain, _, _, err := n.getDriverChains()
		if err != nil {
			return fmt.Errorf("Failed to setup IP tables, cannot acquire chain info %s", err.Error())
		}

		err = iptables.ProgramChain(natChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program NAT chain: %s", err.Error())
		}

		err = iptables.ProgramChain(filterChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program FILTER chain: %s", err.Error())
		}

		n.registerIptCleanFunc(func() error {
			return iptables.ProgramChain(filterChain, config.BridgeName, hairpinMode, false)
		})

		n.portMapper.SetIptablesChain(natChain, n.getNetworkBridgeName())
	}

	d.Lock()
	err = iptables.EnsureJumpRule("FORWARD", IsolationChain1)
	d.Unlock()
	if err != nil {
		return err
	}

	return nil
}

type iptRule struct {
	table   iptables.Table
	chain   string
	preArgs []string
	args    []string
}

func setupIPTablesInternal(bridgeIface string, addr net.Addr, icc, ipmasq, hairpin, enable bool) error {

	var (
		address   = addr.String()
		natRule   = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-s", address, "!", "-o", bridgeIface, "-j", "MASQUERADE"}}
		hpNatRule = iptRule{table: iptables.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", bridgeIface, "-j", "MASQUERADE"}}
		skipDNAT  = iptRule{table: iptables.Nat, chain: DockerChain, preArgs: []string{"-t", "nat"}, args: []string{"-i", bridgeIface, "-j", "RETURN"}}
		outRule   = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}}
	)

	// Set NAT.
	if ipmasq {
		if err := programChainRule(natRule, "NAT", enable); err != nil {
			return err
		}
	}

	if ipmasq && !hairpin {
		if err := programChainRule(skipDNAT, "SKIP DNAT", enable); err != nil {
			return err
		}
	}

	// In hairpin mode, masquerade traffic from localhost
	if hairpin {
		if err := programChainRule(hpNatRule, "MASQ LOCAL HOST", enable); err != nil {
			return err
		}
	}

	// Set Inter Container Communication.
	if err := setIcc(bridgeIface, icc, enable); err != nil {
		return err
	}

	// Set Accept on all non-intercontainer outgoing packets.
	return programChainRule(outRule, "ACCEPT NON_ICC OUTGOING", enable)
}

func programChainRule(rule iptRule, ruleDescr string, insert bool) error {
	var (
		prefix    []string
		operation string
		condition bool
		doesExist = iptables.Exists(rule.table, rule.chain, rule.args...)
	)

	if insert {
		condition = !doesExist
		prefix = []string{"-I", rule.chain}
		operation = "enable"
	} else {
		condition = doesExist
		prefix = []string{"-D", rule.chain}
		operation = "disable"
	}
	if rule.preArgs != nil {
		prefix = append(rule.preArgs, prefix...)
	}

	if condition {
		if err := iptables.RawCombinedOutput(append(prefix, rule.args...)...); err != nil {
			return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
		}
	}

	return nil
}

func setIcc(bridgeIface string, iccEnable, insert bool) error {
	var (
		table      = iptables.Filter
		chain      = "FORWARD"
		args       = []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs   = append(args, "DROP")
	)

	if insert {
		if !iccEnable {
			iptables.Raw(append([]string{"-D", chain}, acceptArgs...)...)

			if !iptables.Exists(table, chain, dropArgs...) {
				if err := iptables.RawCombinedOutput(append([]string{"-A", chain}, dropArgs...)...); err != nil {
					return fmt.Errorf("Unable to prevent intercontainer communication: %s", err.Error())
				}
			}
		} else {
			iptables.Raw(append([]string{"-D", chain}, dropArgs...)...)

			if !iptables.Exists(table, chain, acceptArgs...) {
				if err := iptables.RawCombinedOutput(append([]string{"-I", chain}, acceptArgs...)...); err != nil {
					return fmt.Errorf("Unable to allow intercontainer communication: %s", err.Error())
				}
			}
		}
	} else {
		// Remove any ICC rule.
		if !iccEnable {
			if iptables.Exists(table, chain, dropArgs...) {
				iptables.Raw(append([]string{"-D", chain}, dropArgs...)...)
			}
		} else {
			if iptables.Exists(table, chain, acceptArgs...) {
				iptables.Raw(append([]string{"-D", chain}, acceptArgs...)...)
			}
		}
	}

	return nil
}

// Control Inter Network Communication. Install[Remove] only if it is [not] present.
func setINC(iface string, enable bool) error {
	var (
		action    = iptables.Insert
		actionMsg = "add"
		chains    = []string{IsolationChain1, IsolationChain2}
		rules     = [][]string{
			{"-i", iface, "!", "-o", iface, "-j", IsolationChain2},
			{"-o", iface, "-j", "DROP"},
		}
	)

	if !enable {
		action = iptables.Delete
		actionMsg = "remove"
	}

	for i, chain := range chains {
		if err := iptables.ProgramRule(iptables.Filter, chain, action, rules[i]); err != nil {
			msg := fmt.Sprintf("unable to %s inter-network communication rule: %v", actionMsg, err)
			if enable {
				if i == 1 {
					// Rollback the rule installed on first chain
					if err2 := iptables.ProgramRule(iptables.Filter, chains[0], iptables.Delete, rules[0]); err2 != nil {
						logrus.Warn("Failed to rollback iptables rule after failure (%v): %v", err, err2)
					}
				}
				return fmt.Errorf(msg)
			}
			logrus.Warn(msg)
		}
	}

	return nil
}

// Obsolete chain from previous docker versions
const oldIsolationChain = "DOCKER-ISOLATION"

func removeIPChains() {
	// Remove obsolete rules from default chains
	iptables.ProgramRule(iptables.Filter, "FORWARD", iptables.Delete, []string{"-j", oldIsolationChain})

	// Remove chains
	for _, chainInfo := range []iptables.ChainInfo{
		{Name: DockerChain, Table: iptables.Nat},
		{Name: DockerChain, Table: iptables.Filter},
		{Name: IsolationChain1, Table: iptables.Filter},
		{Name: IsolationChain2, Table: iptables.Filter},
		{Name: oldIsolationChain, Table: iptables.Filter},
	} {
		if err := chainInfo.Remove(); err != nil {
			logrus.Warnf("Failed to remove existing iptables entries in table %s chain %s : %v", chainInfo.Table, chainInfo.Name, err)
		}
	}
}

func setupInternalNetworkRules(bridgeIface string, addr net.Addr, icc, insert bool) error {
	var (
		inDropRule  = iptRule{table: iptables.Filter, chain: IsolationChain1, args: []string{"-i", bridgeIface, "!", "-d", addr.String(), "-j", "DROP"}}
		outDropRule = iptRule{table: iptables.Filter, chain: IsolationChain1, args: []string{"-o", bridgeIface, "!", "-s", addr.String(), "-j", "DROP"}}
	)
	if err := programChainRule(inDropRule, "DROP INCOMING", insert); err != nil {
		return err
	}
	if err := programChainRule(outDropRule, "DROP OUTGOING", insert); err != nil {
		return err
	}
	// Set Inter Container Communication.
	return setIcc(bridgeIface, icc, insert)
}

func clearEndpointConnections(nlh *netlink.Handle, ep *bridgeEndpoint) {
	var ipv4List []net.IP
	var ipv6List []net.IP
	if ep.addr != nil {
		ipv4List = append(ipv4List, ep.addr.IP)
	}
	if ep.addrv6 != nil {
		ipv6List = append(ipv6List, ep.addrv6.IP)
	}
	iptables.DeleteConntrackEntries(nlh, ipv4List, ipv6List)
}
