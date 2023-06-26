//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
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

func setupIPForwarding(enableIPTables bool, enableIP6Tables bool) error {
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
			iptable := iptables.GetIptable(iptables.IPv4)
			if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
				if err := configureIPForwarding(false); err != nil {
					log.G(context.TODO()).Errorf("Disabling IP forwarding failed, %v", err)
				}
				return err
			}
			iptables.OnReloaded(func() {
				log.G(context.TODO()).Debug("Setting the default DROP policy on firewall reload")
				if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
					log.G(context.TODO()).Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
				}
			})
		}
	}

	// add only iptables rules - forwarding is handled by setupIPv6Forwarding in setup_ipv6
	if enableIP6Tables {
		iptable := iptables.GetIptable(iptables.IPv6)
		if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
			log.G(context.TODO()).Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
		}
		iptables.OnReloaded(func() {
			log.G(context.TODO()).Debug("Setting the default DROP policy on firewall reload")
			if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
				log.G(context.TODO()).Warnf("Setting the default DROP policy on firewall reload failed, %v", err)
			}
		})
	}

	return nil
}
