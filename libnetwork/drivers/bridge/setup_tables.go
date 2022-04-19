package bridge

import (
	"errors"
	"fmt"
	"net"

	"github.com/docker/docker/libnetwork/conntrack"
	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
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

func setupIPChains(config *configuration, version firewallapi.IPVersion) (firewallapi.FirewallChain, firewallapi.FirewallChain, firewallapi.FirewallChain, firewallapi.FirewallChain, error) {
	// Sanity check.
	if config.EnableIPTables == false {
		return nil, nil, nil, nil, errors.New("cannot create new chains, EnableIPTable is disabled")
	}

	var table firewallapi.FirewallTable

	if config.EnableNFTables {
		table = nftables.GetTable(nftables.IPVersion(version))
	} else {
		table = iptables.GetTable(iptables.IPVersion(version))
	}

	hairpinMode := !config.EnableUserlandProxy

	natChain, err := table.NewChain(DockerChain, iptables.Nat, hairpinMode)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create NAT chain %s: %v", DockerChain, err)
	}
	defer func() {
		if err != nil {
			if err := table.RemoveExistingChain(DockerChain, iptables.Nat); err != nil {
				logrus.Warnf("failed on removing iptables NAT chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	filterChain, err := table.NewChain(DockerChain, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER chain %s: %v", DockerChain, err)
	}
	defer func() {
		if err != nil {
			if err := table.RemoveExistingChain(DockerChain, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", DockerChain, err)
			}
		}
	}()

	isolationChain1, err := table.NewChain(IsolationChain1, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if err != nil {
			if err := table.RemoveExistingChain(IsolationChain1, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain1, err)
			}
		}
	}()

	isolationChain2, err := table.NewChain(IsolationChain2, iptables.Filter, false)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if err != nil {
			if err := table.RemoveExistingChain(IsolationChain2, iptables.Filter); err != nil {
				logrus.Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", IsolationChain2, err)
			}
		}
	}()

	if err := table.AddReturnRule(IsolationChain1); err != nil {
		return nil, nil, nil, nil, err
	}

	if err := table.AddReturnRule(IsolationChain2); err != nil {
		return nil, nil, nil, nil, err
	}

	return natChain, filterChain, isolationChain1, isolationChain2, nil
}

func (n *bridgeNetwork) setupIP4Tables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableIPTables {
		return errors.New("Cannot program chains, EnableIPTable is disabled")
	}

	maskedAddrv4 := &net.IPNet{
		IP:   i.bridgeIPv4.IP.Mask(i.bridgeIPv4.Mask),
		Mask: i.bridgeIPv4.Mask,
	}
	return n.setupFirewallTables(iptables.IPv4, maskedAddrv4, config, i)
}

func (n *bridgeNetwork) setupIP6Tables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableIP6Tables {
		return errors.New("Cannot program chains, EnableIP6Tables is disabled")
	}

	maskedAddrv6 := &net.IPNet{
		IP:   i.bridgeIPv6.IP.Mask(i.bridgeIPv6.Mask),
		Mask: i.bridgeIPv6.Mask,
	}

	return n.setupFirewallTables(iptables.IPv6, maskedAddrv6, config, i)
}

func (n *bridgeNetwork) setupNFTables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableNFTables {
		return errors.New("Cannot program chains, EnableNFTables is disabled")
	}

	maskedAddrv4 := &net.IPNet{
		IP:   i.bridgeIPv4.IP.Mask(i.bridgeIPv4.Mask),
		Mask: i.bridgeIPv4.Mask,
	}
	return n.setupFirewallTables(nftables.IPv4, maskedAddrv4, config, i)
}

func (n *bridgeNetwork) setupNF6Tables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if !driverConfig.EnableNFTables {
		return errors.New("Cannot program chains for ipv6, EnableNFTables is disabled")
	}

	maskedAddrv6 := &net.IPNet{
		IP:   i.bridgeIPv6.IP.Mask(i.bridgeIPv6.Mask),
		Mask: i.bridgeIPv6.Mask,
	}

	return n.setupFirewallTables(nftables.IPv6, maskedAddrv6, config, i)
}

func (n *bridgeNetwork) setupFirewallTables(ipVersion firewallapi.IPVersion, maskedAddr *net.IPNet, config *networkConfiguration, i *bridgeInterface) error {
	var err error

	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Pickup this configuration option from driver
	hairpinMode := !driverConfig.EnableUserlandProxy

	var table firewallapi.FirewallTable

	if driverConfig.EnableNFTables {
		table = nftables.GetTable(nftables.IPVersion(ipVersion))
	} else {
		table = iptables.GetTable(iptables.IPVersion(ipVersion))
	}

	if config.Internal {
		if err = setupInternalNetworkRules(config.BridgeName, maskedAddr, config.EnableICC, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupInternalNetworkRules(config.BridgeName, maskedAddr, config.EnableICC, false)
		})
	} else {
		if err = setupIPTablesInternal(config.HostIP, config.BridgeName, maskedAddr, config.EnableICC, config.EnableIPMasquerade, hairpinMode, true); err != nil {
			return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
		}
		n.registerIptCleanFunc(func() error {
			return setupIPTablesInternal(config.HostIP, config.BridgeName, maskedAddr, config.EnableICC, config.EnableIPMasquerade, hairpinMode, false)
		})
		natChain, filterChain, _, _, err := n.getDriverChains(ipVersion)
		if err != nil {
			return fmt.Errorf("Failed to setup IP tables, cannot acquire chain info %s", err.Error())
		}

		err = table.ProgramChain(natChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program NAT chain: %s", err.Error())
		}

		err = table.ProgramChain(filterChain, config.BridgeName, hairpinMode, true)
		if err != nil {
			return fmt.Errorf("Failed to program FILTER chain: %s", err.Error())
		}

		n.registerIptCleanFunc(func() error {
			return table.ProgramChain(filterChain, config.BridgeName, hairpinMode, false)
		})

		if ipVersion == iptables.IPv4 {
			n.portMapper.SetFirewallTablesChain(natChain, n.getNetworkBridgeName(), table)
		} else {
			n.portMapperV6.SetFirewallTablesChain(natChain, n.getNetworkBridgeName(), table)
		}
	}

	d.Lock()
	err = table.EnsureJumpRule("FORWARD", IsolationChain1)
	d.Unlock()
	return err
}

type firewallRule struct {
	table   firewallapi.Table
	chain   string
	preArgs []string
	args    []string
}

func setupIPTablesInternal(hostIP net.IP, bridgeIface string, addr *net.IPNet, icc, ipmasq, hairpin, enable bool) error {

	var (
		address   = addr.String()
		skipDNAT  = firewallRule{table: firewallapi.Nat, chain: DockerChain, preArgs: []string{"-t", "nat"}, args: []string{"-i", bridgeIface, "-j", "RETURN"}}
		outRule   = firewallRule{table: firewallapi.Filter, chain: "FORWARD", args: []string{"-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}}
		natArgs   []string
		hpNatArgs []string
	)
	// if hostIP is set use this address as the src-ip during SNAT
	if hostIP != nil {
		hostAddr := hostIP.String()
		natArgs = []string{"-s", address, "!", "-o", bridgeIface, "-j", "SNAT", "--to-source", hostAddr}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", bridgeIface, "-j", "SNAT", "--to-source", hostAddr}
		// Else use MASQUERADE which picks the src-ip based on NH from the route table
	} else {
		natArgs = []string{"-s", address, "!", "-o", bridgeIface, "-j", "MASQUERADE"}
		hpNatArgs = []string{"-m", "addrtype", "--src-type", "LOCAL", "-o", bridgeIface, "-j", "MASQUERADE"}
	}

	natRule := firewallRule{table: firewallapi.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: natArgs}
	hpNatRule := firewallRule{table: firewallapi.Nat, chain: "POSTROUTING", preArgs: []string{"-t", "nat"}, args: hpNatArgs}

	_, ipVersion, nftablesEnabled := getFirewallTableImplementationByAddr(addr)

	if nftablesEnabled {
		skipDNAT.preArgs = []string{"nat"}
		skipDNAT.args = []string{"iifname", bridgeIface, "return"}
		outRule.args = []string{"iifname", bridgeIface, "oifname", "!=", bridgeIface, "accept"}

		if hostIP != nil {
			hostAddr := hostIP.String()
			natArgs = []string{"oifname", "!=", bridgeIface, "ip", "saddr", address, "snat", "to", hostAddr}
			hpNatArgs = []string{"oifname", bridgeIface, "fib", "saddr", "type", "local", "snat", "to", hostAddr}
		} else {
			natArgs = []string{"oifname", "!=", bridgeIface, "ip", "saddr", address, "masquerade"}
			hpNatArgs = []string{"oifname", bridgeIface, "fib", "saddr", "type", "local", "masquerade"}
		}
	}

	// Set NAT.
	if ipmasq {
		if err := programChainRule(ipVersion, natRule, "NAT", enable); err != nil {
			return err
		}
	}

	if ipmasq && !hairpin {
		if err := programChainRule(ipVersion, skipDNAT, "SKIP DNAT", enable); err != nil {
			return err
		}
	}

	// In hairpin mode, masquerade traffic from localhost
	if hairpin {
		if err := programChainRule(ipVersion, hpNatRule, "MASQ LOCAL HOST", enable); err != nil {
			return err
		}
	}

	// Set Inter Container Communication.
	if err := setIcc(ipVersion, bridgeIface, icc, enable); err != nil {
		return err
	}

	// Set Accept on all non-intercontainer outgoing packets.
	return programChainRule(ipVersion, outRule, "ACCEPT NON_ICC OUTGOING", enable)
}

func programChainRule(version firewallapi.IPVersion, rule firewallRule, ruleDescr string, insert bool) error {
	var table firewallapi.FirewallTable
	var nftablesEnabled bool
	if err := nftables.InitCheck(); err == nil {
		table = nftables.GetTable(version)
	} else {
		table = iptables.GetTable(version)
	}

	var (
		prefix    []string
		operation string
		condition bool
		doesExist = table.Exists(rule.table, rule.chain, rule.args...)
	)

	if insert {
		condition = !doesExist
		prefix = []string{table.GetInsertAction(), rule.chain}
		operation = "enable"
	} else {
		condition = doesExist
		prefix = []string{table.GetDeleteAction(), rule.chain}
		operation = "disable"
	}
	if rule.preArgs != nil {
		if nftablesEnabled {
			prefix = append([]string{prefix[0], string(version)}, rule.preArgs...)
		} else {
			prefix = append(rule.preArgs, prefix...)
		}
	}

	if condition {
		if operation == "disable" && doesExist {
			if err := table.DeleteRule(version, rule.table, rule.chain, rule.args...); err != nil {
				return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
			}
		} else {
			if err := table.RawCombinedOutput(append(prefix, rule.args...)...); err != nil {
				return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
			}
		}
	}

	return nil
}

func setIcc(version firewallapi.IPVersion, bridgeIface string, iccEnable, insert bool) error {
	fwtable, nftablesEnabled := getFirewallTableImplementationByVersion(version)

	var (
		table      = firewallapi.Filter
		chain      = "FORWARD"
		args       = []string{}
		acceptArgs = []string{}
		dropArgs   = []string{}
	)

	if nftablesEnabled {
		args = []string{"iifname", bridgeIface, "oifname", bridgeIface}
		acceptArgs = append(args, "accept")
		dropArgs = append(args, "drop")
	} else {
		args = []string{"-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs = append(args, "DROP")
	}

	if insert {
		if !iccEnable {
			fwtable.DeleteRule(version, table, chain, acceptArgs...)

			if !fwtable.Exists(table, chain, dropArgs...) {
				if err := fwtable.ProgramRule(table, chain, firewallapi.Action(fwtable.GetAppendAction()), dropArgs); err != nil {
					return fmt.Errorf("Unable to prevent intercontainer communication: %s", err.Error())
				}
			}
		} else {
			fwtable.DeleteRule(version, table, chain, dropArgs...)

			if !fwtable.Exists(table, chain, acceptArgs...) {
				if err := fwtable.ProgramRule(table, chain, firewallapi.Action(fwtable.GetInsertAction()), acceptArgs); err != nil {
					return fmt.Errorf("Unable to allow intercontainer communication: %s", err.Error())
				}
			}
		}
	} else {
		// Remove any ICC rule.
		if !iccEnable {
			if fwtable.Exists(table, chain, dropArgs...) {
				fwtable.Raw(append([]string{"-D", chain}, dropArgs...)...)
			}
		} else {
			if fwtable.Exists(table, chain, acceptArgs...) {
				fwtable.Raw(append([]string{"-D", chain}, acceptArgs...)...)
			}
		}
	}

	return nil
}

// Control Inter Network Communication. Install[Remove] only if it is [not] present.
func setINC(version firewallapi.IPVersion, iface string, enable bool) error {
	table, nftablesEnabled := getFirewallTableImplementationByVersion(version)
	var (
		action    = firewallapi.Action(table.GetInsertAction())
		actionMsg = "add"
		chains    = []string{IsolationChain1, IsolationChain2}
		rules     = [][]string{
			{"-i", iface, "!", "-o", iface, "-j", IsolationChain2},
			{"-o", iface, "-j", "DROP"},
		}
	)

	if !enable {
		action = firewallapi.Action(table.GetDeleteAction())
		actionMsg = "remove"
	}

	if nftablesEnabled {
		rules = [][]string{
			{"iifname", iface, "oifname", "!=", iface, "jump", IsolationChain1},
			{"oifname", iface, "drop"},
		}
	}

	for i, chain := range chains {
		var err error
		if !enable {
			err = table.DeleteRule(version, firewallapi.Filter, chain, rules[i]...)
		} else {
			err = table.ProgramRule(firewallapi.Filter, chain, action, rules[i])
		}
		if err != nil {
			msg := fmt.Sprintf("unable to %s inter-network communication rule: %v", actionMsg, err)
			if enable {
				if i == 1 {
					// Rollback the rule installed on first chain
					if err2 := table.ProgramRule(firewallapi.Filter, chains[0], iptables.Delete, rules[0]); err2 != nil {
						logrus.Warnf("Failed to rollback firewall rule after failure (%v): %v", err, err2)
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

func removeIPChains(version firewallapi.IPVersion) {
	var table firewallapi.FirewallTable
	if err := nftables.InitCheck(); err == nil {
		table = nftables.GetTable(version)
		// Remove obsolete rules from default chain
		table.DeleteRule(version, firewallapi.Filter, "FORWARD", "jump", oldIsolationChain)
	} else {
		table = iptables.GetTable(version)
		// Remove obsolete rules from default chains
		table.DeleteRule(version, firewallapi.Filter, "FORWARD", "-j", oldIsolationChain)
	}

	// Remove chains
	table.RemoveExistingChain(DockerChain, firewallapi.Nat)
	table.RemoveExistingChain(DockerChain, firewallapi.Filter)
	table.RemoveExistingChain(IsolationChain1, firewallapi.Filter)
	table.RemoveExistingChain(IsolationChain2, firewallapi.Filter)
	table.RemoveExistingChain(oldIsolationChain, firewallapi.Filter)
}

func setupInternalNetworkRules(bridgeIface string, addr *net.IPNet, icc, insert bool) error {
	_, version, nftablesEnabled := getFirewallTableImplementationByAddr(addr)

	var (
		inDropRule  = firewallRule{table: firewallapi.Filter, chain: IsolationChain1, args: []string{"-i", bridgeIface, "!", "-d", addr.String(), "-j", "DROP"}}
		outDropRule = firewallRule{table: firewallapi.Filter, chain: IsolationChain1, args: []string{"-o", bridgeIface, "!", "-s", addr.String(), "-j", "DROP"}}
	)

	if nftablesEnabled {
		inDropRule.args = []string{"iifname", bridgeIface, "ip", "daddr", "!=", addr.String(), "drop"}
		outDropRule.args = []string{"oifname", bridgeIface, "ip", "saddr", "!=", addr.String(), "drop"}
	}

	if err := programChainRule(version, inDropRule, "DROP INCOMING", insert); err != nil {
		return err
	}
	if err := programChainRule(version, outDropRule, "DROP OUTGOING", insert); err != nil {
		return err
	}
	// Set Inter Container Communication.
	return setIcc(version, bridgeIface, icc, insert)
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
	conntrack.DeleteConntrackEntries(nlh, ipv4List, ipv6List)
}

func getFirewallTableImplementationByAddr(addr *net.IPNet) (firewallapi.FirewallTable, firewallapi.IPVersion, bool) {
	var table firewallapi.FirewallTable
	var version firewallapi.IPVersion
	var nftablesEnabled bool
	if err := nftables.InitCheck(); err == nil {
		if addr.IP.To4() == nil {
			version = nftables.IPv6
		} else {
			version = nftables.IPv4
		}
		table = nftables.GetTable(version)
		nftablesEnabled = true
	} else {
		if addr.IP.To4() == nil {
			version = iptables.IPv6
		} else {
			version = iptables.IPv4
		}
		table = iptables.GetTable(version)
		nftablesEnabled = false
	}
	return table, version, nftablesEnabled
}

func getFirewallTableImplementationByVersion(version firewallapi.IPVersion) (firewallapi.FirewallTable, bool) {
	var table firewallapi.FirewallTable
	var nftablesEnabled bool
	if err := nftables.InitCheck(); err == nil {
		table = nftables.GetTable(version)
		nftablesEnabled = true
	} else {

		table = iptables.GetTable(version)
		nftablesEnabled = false
	}
	return table, nftablesEnabled
}
