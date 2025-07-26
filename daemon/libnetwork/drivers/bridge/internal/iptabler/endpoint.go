//go:build linux

package iptabler

import (
	"context"
	"net/netip"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
)

func (n *network) AddEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	return n.modEndpoint(ctx, epIPv4, epIPv6, true)
}

func (n *network) DelEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	return n.modEndpoint(ctx, epIPv4, epIPv6, false)
}

func (n *network) modEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr, enable bool) error {
	if n.ipt.config.IPv4 && epIPv4.IsValid() {
		if err := n.filterDirectAccess(ctx, iptables.IPv4, n.config.Config4, epIPv4, enable); err != nil {
			return err
		}
	}
	if n.ipt.config.IPv6 && epIPv6.IsValid() {
		if err := n.filterDirectAccess(ctx, iptables.IPv6, n.config.Config6, epIPv6, enable); err != nil {
			return err
		}
	}
	return nil
}

// filterDirectAccess drops packets addressed directly to the container's IP address,
// when direct routing is not permitted by network configuration.
//
// It is a no-op if:
//   - the network is internal
//   - gateway mode is "nat-unprotected" or "routed".
//   - direct routing is enabled at the daemon level.
//   - "raw" rules are disabled (possibly because the host doesn't have the necessary
//     kernel support).
//
// Packets originating on the bridge's own interface and addressed directly to the
// container are allowed - the host always has direct access to its own containers
// (it doesn't need to use the port mapped to its own addresses, although it can).
//
// "Trusted interfaces" are treated in the same way as the bridge itself.
func (n *network) filterDirectAccess(ctx context.Context, ipv iptables.IPVersion, config firewaller.NetworkConfigFam, epIP netip.Addr, enable bool) error {
	if n.config.Internal || config.Unprotected || config.Routed {
		return nil
	}
	// For config that may change between daemon restarts, make sure rules are
	// removed - if the container was left running when the daemon stopped, and
	// direct routing has since been disabled, the rules need to be deleted when
	// cleanup happens on restart. This also means a change in config over a
	// live-restore restart will take effect.
	if n.ipt.config.AllowDirectRouting || rawRulesDisabled(ctx) {
		enable = false
	}
	for _, ifName := range n.config.TrustedHostInterfaces {
		accept := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
			"-d", epIP.String(),
			"-i", ifName,
			"-j", "ACCEPT",
		}}
		if err := appendOrDelChainRule(accept, "DIRECT ACCESS FILTERING - ACCEPT", enable); err != nil {
			return err
		}
	}
	accept := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-d", epIP.String(),
		"!", "-i", n.config.IfName,
		"-j", "DROP",
	}}
	return appendOrDelChainRule(accept, "DIRECT ACCESS FILTERING - DROP", enable)
}
