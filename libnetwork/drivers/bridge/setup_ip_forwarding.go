//go:build linux

package bridge

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
)

const (
	forwardConfPerm        = 0o644
	ipv4ForwardConf        = "/proc/sys/net/ipv4/ip_forward"
	ipv6ForwardConfDefault = "/proc/sys/net/ipv6/conf/default/forwarding"
	ipv6ForwardConfAll     = "/proc/sys/net/ipv6/conf/all/forwarding"
)

func setupIPv4Forwarding(wantFilterForwardDrop bool) (retErr error) {
	// Get current IPv4 forward setup
	ipv4ForwardData, err := os.ReadFile(ipv4ForwardConf)
	if err != nil {
		return fmt.Errorf("Cannot read IP forwarding setup: %v", err)
	}

	// Enable IPv4 forwarding only if it is not already enabled
	if ipv4ForwardData[0] != '1' {
		if err := os.WriteFile(ipv4ForwardConf, []byte{'1', '\n'}, forwardConfPerm); err != nil {
			return fmt.Errorf("Enabling IP forwarding failed: %v", err)
		}
		defer func() {
			if retErr != nil {
				if err := os.WriteFile(ipv4ForwardConf, []byte{'0', '\n'}, forwardConfPerm); err != nil {
					log.G(context.TODO()).WithError(err).Error("Disabling IPv4 forwarding failed")
				}
			}
		}()

		// When enabling ip_forward set the default policy on forward chain to drop.
		if wantFilterForwardDrop {
			if err := setFilterForwardDrop(iptables.IPv4); err != nil {
				return err
			}
		}
	}
	return nil
}

func setupIPv6Forwarding(wantFilterForwardDrop bool) (retErr error) {
	configUpdated := false

	// Get current IPv6 default forwarding setup
	// FIXME(robmry) - is it necessary to set this, setting "all" (below) does the job?
	ipv6ForwardDataDefault, err := os.ReadFile(ipv6ForwardConfDefault)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 default forwarding setup: %v", err)
	}
	// Enable IPv6 default forwarding only if it is not already enabled
	if ipv6ForwardDataDefault[0] != '1' {
		if err := os.WriteFile(ipv6ForwardConfDefault, []byte{'1', '\n'}, forwardConfPerm); err != nil {
			log.G(context.TODO()).Warnf("Unable to enable IPv6 default forwarding: %v", err)
		}
		configUpdated = true
	}
	defer func() {
		if retErr != nil {
			if err := os.WriteFile(ipv6ForwardConfDefault, []byte{'0', '\n'}, forwardConfPerm); err != nil {
				log.G(context.TODO()).WithError(err).Error("Disabling IPv6 default forwarding failed")
			}
		}
	}()

	// Get current IPv6 all forwarding setup
	ipv6ForwardDataAll, err := os.ReadFile(ipv6ForwardConfAll)
	if err != nil {
		return fmt.Errorf("Cannot read IPv6 all forwarding setup: %v", err)
	}
	// Enable IPv6 all forwarding only if it is not already enabled
	if ipv6ForwardDataAll[0] != '1' {
		if err := os.WriteFile(ipv6ForwardConfAll, []byte{'1', '\n'}, forwardConfPerm); err != nil {
			log.G(context.TODO()).Warnf("Unable to enable IPv6 all forwarding: %v", err)
		}
		configUpdated = true
	}
	defer func() {
		if retErr != nil {
			if err := os.WriteFile(ipv6ForwardConfAll, []byte{'0', '\n'}, forwardConfPerm); err != nil {
				log.G(context.TODO()).WithError(err).Error("Disabling IPv6 all forwarding failed")
			}
		}
	}()

	if configUpdated && wantFilterForwardDrop {
		if err := setFilterForwardDrop(iptables.IPv6); err != nil {
			return err
		}
	}

	return nil
}

func setFilterForwardDrop(ipv iptables.IPVersion) error {
	iptable := iptables.GetIptable(ipv)
	if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
		return err
	}
	iptables.OnReloaded(func() {
		log.G(context.TODO()).WithFields(log.Fields{"ipv": ipv}).Debug("Setting the default DROP policy on firewall reload")
		if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
			log.G(context.TODO()).WithFields(log.Fields{
				"error": err,
				"ipv":   ipv,
			}).Warn("Failed to set the default DROP policy on firewall reload")
		}
	})
	return nil
}
