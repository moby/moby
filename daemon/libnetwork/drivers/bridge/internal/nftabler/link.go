//go:build linux

package nftabler

import (
	"context"
	"errors"
	"fmt"
	"net/netip"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func (n *network) AddLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) error {
	if !parentIP.IsValid() || parentIP.IsUnspecified() {
		return errors.New("cannot link to a container with an empty parent IP address")
	}
	if !childIP.IsValid() || childIP.IsUnspecified() {
		return errors.New("cannot link to a container with an empty child IP address")
	}

	chain := n.fw.table4.Chain(ctx, chainFilterFwdIn(n.config.IfName))
	for _, port := range ports {
		for _, rule := range legacyLinkRules(parentIP, childIP, port) {
			if err := chain.AppendRule(ctx, fwdInLegacyLinksRuleGroup, rule); err != nil {
				return err
			}
		}
	}
	if err := nftApply(ctx, n.fw.table4); err != nil {
		return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
	}
	return nil
}

func (n *network) DelLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) {
	chain := n.fw.table4.Chain(ctx, chainFilterFwdIn(n.config.IfName))
	for _, port := range ports {
		for _, rule := range legacyLinkRules(parentIP, childIP, port) {
			if err := chain.DeleteRule(ctx, fwdInLegacyLinksRuleGroup, rule); err != nil {
				log.G(ctx).WithFields(log.Fields{
					"rule":  rule,
					"error": err,
				}).Warn("Failed to remove link between containers")
			}
		}
	}
	if err := nftApply(ctx, n.fw.table4); err != nil {
		log.G(ctx).WithError(err).Warn("Removing link, failed to update nftables")
	}
}

func legacyLinkRules(parentIP, childIP netip.Addr, port types.TransportPort) []string {
	// TODO(robmry) - could combine rules for each proto by using an anonymous set.
	return []string{
		// Match the iptables implementation, but without checking iifname/oifname (not needed
		// because the addresses belong to the bridge).
		fmt.Sprintf("ip saddr %s ip daddr %s %s dport %d counter accept", parentIP.Unmap(), childIP.Unmap(), port.Proto, port.Port),
		// Conntrack will allow responses. So, this must be to allow unsolicited packets from an exposed port.
		fmt.Sprintf("ip daddr %s ip saddr %s %s sport %d counter accept", parentIP.Unmap(), childIP.Unmap(), port.Proto, port.Port),
	}
}
