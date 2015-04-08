package bridge

import (
	"fmt"
	"net"

	"github.com/docker/docker/pkg/iptables"
	"github.com/docker/libnetwork"
	"github.com/docker/libnetwork/portmapper"
)

// DockerChain: DOCKER iptable chain name
const (
	DockerChain = "DOCKER"
)

func setupIPTables(i *bridgeInterface) error {
	// Sanity check.
	if i.Config.EnableIPTables == false {
		return fmt.Errorf("Unexpected request to set IP tables for interface: %s", i.Config.BridgeName)
	}

	addrv4, _, err := libnetwork.GetIfaceAddr(i.Config.BridgeName)
	if err != nil {
		return fmt.Errorf("Failed to setup IP tables, cannot acquire Interface address: %s", err.Error())
	}
	if err = setupIPTablesInternal(i.Config.BridgeName, addrv4, i.Config.EnableICC, i.Config.EnableIPMasquerade, true); err != nil {
		return fmt.Errorf("Failed to Setup IP tables: %s", err.Error())
	}

	_, err = iptables.NewChain(DockerChain, i.Config.BridgeName, iptables.Nat)
	if err != nil {
		return fmt.Errorf("Failed to create NAT chain: %s", err.Error())
	}

	chain, err := iptables.NewChain(DockerChain, i.Config.BridgeName, iptables.Filter)
	if err != nil {
		return fmt.Errorf("Failed to create FILTER chain: %s", err.Error())
	}

	portmapper.SetIptablesChain(chain)

	return nil
}

func setupIPTablesInternal(bridgeIface string, addr net.Addr, icc, ipmasq, enable bool) error {
	var (
		address = addr.String()
		natRule = []string{"POSTROUTING", "-t", "nat", "-s", address, "!", "-o", bridgeIface, "-j", "MASQUERADE"}
		outRule = []string{"FORWARD", "-i", bridgeIface, "!", "-o", bridgeIface, "-j", "ACCEPT"}
		inRule  = []string{"FORWARD", "-o", bridgeIface, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"}
	)

	// Set NAT.
	if ipmasq {
		if err := programChainRule(natRule, "NAT", enable); err != nil {
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

func programChainRule(ruleArgs []string, ruleDescr string, insert bool) error {
	var (
		prefix    []string
		operation string
		condition bool
	)

	if insert {
		condition = !iptables.Exists(ruleArgs...)
		prefix = []string{"-I"}
		operation = "enable"
	} else {
		condition = iptables.Exists(ruleArgs...)
		prefix = []string{"-D"}
		operation = "disable"
	}

	if condition {
		if output, err := iptables.Raw(append(prefix, ruleArgs...)...); err != nil {
			return fmt.Errorf("Unable to %s %s rule: %s", operation, ruleDescr, err.Error())
		} else if len(output) != 0 {
			return &iptables.ChainError{Chain: ruleDescr, Output: output}
		}
	}

	return nil
}

func setIcc(bridgeIface string, iccEnable, insert bool) error {
	var (
		args       = []string{"FORWARD", "-i", bridgeIface, "-o", bridgeIface, "-j"}
		acceptArgs = append(args, "ACCEPT")
		dropArgs   = append(args, "DROP")
	)

	if insert {
		if !iccEnable {
			iptables.Raw(append([]string{"-D"}, acceptArgs...)...)

			if !iptables.Exists(dropArgs...) {
				if output, err := iptables.Raw(append([]string{"-I"}, dropArgs...)...); err != nil {
					return fmt.Errorf("Unable to prevent intercontainer communication: %s", err.Error())
				} else if len(output) != 0 {
					return fmt.Errorf("Error disabling intercontainer communication: %s", output)
				}
			}
		} else {
			iptables.Raw(append([]string{"-D"}, dropArgs...)...)

			if !iptables.Exists(acceptArgs...) {
				if output, err := iptables.Raw(append([]string{"-I"}, acceptArgs...)...); err != nil {
					return fmt.Errorf("Unable to allow intercontainer communication: %s", err.Error())
				} else if len(output) != 0 {
					return fmt.Errorf("Error enabling intercontainer communication: %s", output)
				}
			}
		}
	} else {
		// Remove any ICC rule.
		if !iccEnable {
			if iptables.Exists(dropArgs...) {
				iptables.Raw(append([]string{"-D"}, dropArgs...)...)
			}
		} else {
			if iptables.Exists(acceptArgs...) {
				iptables.Raw(append([]string{"-D"}, acceptArgs...)...)
			}
		}
	}

	return nil
}
