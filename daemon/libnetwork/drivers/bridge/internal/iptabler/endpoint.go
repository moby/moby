//go:build linux

package iptabler

import (
	"context"
	"net/netip"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
)

func (n *network) AddEndpoint(_ context.Context, _, _ netip.Addr) error {
	return nil
}

func (n *network) DelEndpoint(_ context.Context, _, _ netip.Addr) error {
	return nil
}

// deleteLegacyDirectAccess removes per-container raw PREROUTING rules that may
// have been created by an older daemon version. Current versions use a single
// subnet-level rule per network instead (see setSubnetProtection).
func (n *network) deleteLegacyDirectAccess(ctx context.Context, ipv iptables.IPVersion, config firewaller.NetworkConfigFam, epIP netip.Addr) error {
	if n.config.Internal || config.Unprotected || config.Routed {
		return nil
	}
	for _, ifName := range n.config.TrustedHostInterfaces {
		accept := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
			"-d", epIP.String(),
			"-i", ifName,
			"-j", "ACCEPT",
		}}
		if err := appendOrDelChainRule(accept, "DIRECT ACCESS FILTERING - ACCEPT", false); err != nil {
			return err
		}
	}
	drop := iptables.Rule{IPVer: ipv, Table: iptables.Raw, Chain: "PREROUTING", Args: []string{
		"-d", epIP.String(),
		"!", "-i", n.config.IfName,
		"-j", "DROP",
	}}
	return appendOrDelChainRule(drop, "DIRECT ACCESS FILTERING - DROP", false)
}
