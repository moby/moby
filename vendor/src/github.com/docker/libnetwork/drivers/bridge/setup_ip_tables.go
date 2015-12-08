package bridge

import (
	"fmt"
	"net"

	"github.com/Sirupsen/logrus"
	"github.com/docker/libnetwork/iptables"
	"github.com/docker/libnetwork/netutils"
)

// DockerChain: DOCKER iptable chain name
const (
	DockerChain = "DOCKER"
)

func setupIPChains(config *configuration) (*iptables.ChainInfo, *iptables.ChainInfo, error) {
	// Sanity check.
	if config.EnableIPTables == false {
		return nil, nil, fmt.Errorf("Cannot create new chains, EnableIPTable is disabled")
	}

	hairpinMode := !config.EnableUserlandProxy

	natChain, err := iptables.NewChain(DockerChain, iptables.Nat, hairpinMode)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create NAT chain: %s", err.Error())
	}
	defer func() {
		if err != nil {
			if err := iptables.RemoveExistingChain(DockerChain, iptables.Nat); err != nil {
				logrus.Warnf("Failed on removing iptables NAT chain on cleanup: %v", err)
			}
		}
	}()

	filterChain, err := iptables.NewChain(DockerChain, iptables.Filter, hairpinMode)
	if err != nil {
		return nil, nil, fmt.Errorf("Failed to create FILTER chain: %s", err.Error())
	}

	return natChain, filterChain, nil
}

func (n *bridgeNetwork) setupIPTables(config *networkConfiguration, i *bridgeInterface) error {
	d := n.driver
	d.Lock()
	driverConfig := d.config
	d.Unlock()

	// Sanity check.
	if driverConfig.EnableIPTables == false {
		return fmt.Errorf("Cannot program chains, EnableIPTable is disabled")
	}

	// Pickup this configuraton option from driver
	hairpinMode := !driverConfig.EnableUserlandProxy

	addrv4, _, err := netutils.GetIfaceAddr(config.BridgeName)
	if err != nil {
		return fmt.Errorf("Failed to setup IP tables, cannot acquire Interface address: %s", err.Error())
	}
	ipnet := addrv4.(*net.IPNet)
	maskedAddrv4 := &net.IPNet{
		IP:   ipnet.IP.Mask(ipnet.Mask),
		Mask: ipnet.Mask,
	}
	if err = setupIPTablesInternal(config.BridgeName, maskedAddrv4, config.EnableICC, config.EnableIPMasquerade, hairpinMode, true); err != nil {
		return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
	}
	n.registerIptCleanFunc(func() error {
		return setupIPTablesInternal(config.BridgeName, maskedAddrv4, config.EnableICC, config.EnableIPMasquerade, hairpinMode, false)
	})

	natChain, filterChain, err := n.getDriverChains()
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

	n.portMapper.SetIptablesChain(filterChain, n.getNetworkBridgeName())

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
		outRule   = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}}
		inRule    = iptRule{table: iptables.Filter, chain: "FORWARD", args: []string{"-o", bridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}}
	)

	// Set NAT.
	if ipmasq {
		if err := programChainRule(natRule, "NAT", enable); err != nil {
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
	if err := programChainRule(outRule, "ACCEPT NON_ICC OUTGOING", enable); err != nil {
		return err
	}

	// Set Accept on incoming packets for existing connections.
	if err := programChainRule(inRule, "ACCEPT INCOMING", enable); err != nil {
		return err
	}

	return nil
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
		if output, err := iptables.Raw(append(prefix, rule.args...)...); err != nil {
			return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
		} else if len(output) != 0 {
			return &iptables.ChainError{Chain: rule.chain, Output: output}
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
				if output, err := iptables.Raw(append([]string{"-A", chain}, dropArgs...)...); err != nil {
					return fmt.Errorf("Unable to prevent intercontainer communication: %s", err.Error())
				} else if len(output) != 0 {
					return fmt.Errorf("Error disabling intercontainer communication: %s", output)
				}
			}
		} else {
			iptables.Raw(append([]string{"-D", chain}, dropArgs...)...)

			if !iptables.Exists(table, chain, acceptArgs...) {
				if output, err := iptables.Raw(append([]string{"-I", chain}, acceptArgs...)...); err != nil {
					return fmt.Errorf("Unable to allow intercontainer communication: %s", err.Error())
				} else if len(output) != 0 {
					return fmt.Errorf("Error enabling intercontainer communication: %s", output)
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

// Control Inter Network Communication. Install/remove only if it is not/is present.
func setINC(network1, network2 string, enable bool) error {
	var (
		table = iptables.Filter
		chain = "FORWARD"
		args  = [2][]string{{"-s", network1, "-d", network2, "-j", "DROP"}, {"-s", network2, "-d", network1, "-j", "DROP"}}
	)

	if enable {
		for i := 0; i < 2; i++ {
			if iptables.Exists(table, chain, args[i]...) {
				continue
			}
			if output, err := iptables.Raw(append([]string{"-I", chain}, args[i]...)...); err != nil {
				return fmt.Errorf("unable to add inter-network communication rule: %s", err.Error())
			} else if len(output) != 0 {
				return fmt.Errorf("error adding inter-network communication rule: %s", string(output))
			}
		}
	} else {
		for i := 0; i < 2; i++ {
			if !iptables.Exists(table, chain, args[i]...) {
				continue
			}
			if output, err := iptables.Raw(append([]string{"-D", chain}, args[i]...)...); err != nil {
				return fmt.Errorf("unable to remove inter-network communication rule: %s", err.Error())
			} else if len(output) != 0 {
				return fmt.Errorf("error removing inter-network communication rule: %s", string(output))
			}
		}
	}

	return nil
}
