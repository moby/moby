//go:build linux
// +build linux

package bridge

import (
	"fmt"
	"os"

	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/firewalld"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
	"github.com/sirupsen/logrus"
)

const (
	ipv4ForwardConf     = "/proc/sys/net/ipv4/ip_forward"
	ipv4ForwardConfPerm = 0644
)

func configureIPForwarding(enable bool) error {
	var val byte
	if enable {
		val = '1'
	}
	return os.WriteFile(ipv4ForwardConf, []byte{val, '\n'}, ipv4ForwardConfPerm)
}

func setupIPForwarding(enableIPTables bool, enableIP6Tables bool, enableNFTables bool) error {
	var table firewallapi.FirewallTable

	if enableNFTables {
		table = nftables.GetTable(nftables.IPv4)
	} else {
		table = iptables.GetTable(iptables.IPv4)
	}

	// Get current IPv4 forward setup
	ipv4ForwardData, err := os.ReadFile(ipv4ForwardConf)
	if err != nil {
		return fmt.Errorf("Cannot read IP forwarding setup: %v", err)
	}

	// Enable IPv4 forwarding only if it is not already enabled
	if ipv4ForwardData[0] != '1' {
		// Enable IPv4 forwarding
		if err := configureIPForwarding(true); err != nil {
			return fmt.Errorf("Enabling IP forwarding failed: %v", err)
		}
		// When enabling ip_forward set the default policy on forward chain to
		// drop only if the daemon option iptables is not set to false.
		if enableIPTables {
			if err := table.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
				if err := configureIPForwarding(false); err != nil {
					logrus.Errorf("Disabling IP forwarding failed, %v", err)
				}
				return err
			}
			firewalld.OnReloaded(func() {
				logrus.Debug("Setting the default DROP policy on firewall reload")
				if err := table.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
					logrus.Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
				}
			})
		}
	}

	// add only iptables rules - forwarding is handled by setupIPv6Forwarding in setup_ipv6
	if enableIP6Tables {
		if enableNFTables {
			table = nftables.GetTable(nftables.IPv6)
		} else {
			table = iptables.GetTable(iptables.IPv6)
		}
		if err := table.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
			logrus.Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
		}
		firewalld.OnReloaded(func() {
			logrus.Debug("Setting the default DROP policy on firewall reload")
			if err := table.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
				logrus.Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
			}
		})
	}

	return nil
}
