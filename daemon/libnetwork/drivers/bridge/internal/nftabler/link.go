//go:build linux

package nftabler

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strconv"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"github.com/moby/moby/v2/daemon/libnetwork/types"
)

func (n *network) AddLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) error {
	if !parentIP.IsValid() || parentIP.IsUnspecified() {
		return errors.New("cannot link to a container with an empty parent IP address")
	}
	if !childIP.IsValid() || childIP.IsUnspecified() {
		return errors.New("cannot link to a container with an empty child IP address")
	}

	tm := nftables.Modifier{}
	for _, port := range ports {
		updateLegacyLinkRules(tm.Create, chainFilterFwdIn(n.config.IfName), parentIP, childIP, port)
	}
	if err := n.fw.table4.Apply(ctx, tm); err != nil {
		return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
	}
	return nil
}

func (n *network) DelLink(ctx context.Context, parentIP, childIP netip.Addr, ports []types.TransportPort) {
	tm := nftables.Modifier{}
	for _, port := range ports {
		updateLegacyLinkRules(tm.Delete, chainFilterFwdIn(n.config.IfName), parentIP, childIP, port)
	}
	if err := n.fw.table4.Apply(ctx, tm); err != nil {
		log.G(ctx).WithError(err).Warn("Removing link, failed to update nftables")
	}
}

func updateLegacyLinkRules(updater func(command nftables.Obj), chainName string, parentIP, childIP netip.Addr, port types.TransportPort) {
	// TODO(robmry) - could combine rules for each proto by using an anonymous set.
	// Match the iptables implementation, but without checking iifname/oifname (not needed
	// because the addresses belong to the bridge).
	updater(nftables.Rule{
		Chain: chainName,
		Group: fwdInLegacyLinksRuleGroup,
		Rule: []string{
			"ip saddr", parentIP.Unmap().String(),
			"ip daddr", childIP.Unmap().String(), port.Proto.String(), "dport", strconv.Itoa(int(port.Port)),
			"counter accept",
		},
	})
	// Conntrack will allow responses. So, this must be to allow unsolicited packets from an exposed port.
	updater(nftables.Rule{
		Chain: chainName,
		Group: fwdInLegacyLinksRuleGroup,
		Rule: []string{
			"ip saddr", childIP.Unmap().String(), port.Proto.String(), "sport", strconv.Itoa(int(port.Port)),
			"ip daddr", parentIP.Unmap().String(),
			"counter accept",
		},
	})
}
