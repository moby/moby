//go:build linux

package nftabler

import (
	"context"
	"fmt"
	"net/netip"
	"strings"

	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
)

func (n *network) AddEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	if n.fw.cleaner != nil {
		n.fw.cleaner.DelEndpoint(ctx, n.config, epIPv4, epIPv6)
	}
	return n.modEndpoint(ctx, epIPv4, epIPv6, true)
}

func (n *network) DelEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr) error {
	return n.modEndpoint(ctx, epIPv4, epIPv6, false)
}

func (n *network) modEndpoint(ctx context.Context, epIPv4, epIPv6 netip.Addr, enable bool) error {
	if n.fw.config.IPv4 && epIPv4.IsValid() {
		tm := nftables.Modifier{}
		updater := tm.Create
		if !enable {
			updater = tm.Delete
		}
		n.filterDirectAccess(updater, nftables.IPv4, n.config.Config4, epIPv4)
		if err := n.fw.table4.Apply(ctx, tm); err != nil {
			return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
		}
	}
	if n.fw.config.IPv6 && epIPv6.IsValid() {
		tm := nftables.Modifier{}
		updater := tm.Create
		if !enable {
			updater = tm.Delete
		}
		n.filterDirectAccess(updater, nftables.IPv6, n.config.Config6, epIPv6)
		if err := n.fw.table6.Apply(ctx, tm); err != nil {
			return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
		}
	}
	return nil
}

// filterDirectAccess drops packets addressed directly to the container's IP address,
// when direct routing is not permitted by network configuration.
//
// It is a no-op if:
//   - gateway mode is "nat-unprotected" or "routed".
//   - direct routing is enabled at the daemon level.
//   - "raw" rules are disabled (possibly because the host doesn't have the necessary
//     kernel support).
//
// Packets originating on the bridge's own interface and addressed directly to the
// container are allowed - the host always has direct access to its own containers.
// (It doesn't need to use the port mapped to its own addresses, although it can.)
//
// "Trusted interfaces" are treated in the same way as the bridge itself.
func (n *network) filterDirectAccess(updater func(nftables.Obj), fam nftables.Family, conf firewaller.NetworkConfigFam, epIP netip.Addr) {
	if n.config.Internal || conf.Unprotected || conf.Routed || n.fw.config.AllowDirectRouting {
		return
	}
	ifNames := strings.Join(n.config.TrustedHostInterfaces, ", ")
	updater(nftables.Rule{
		Chain: rawPreroutingChain,
		Group: rawPreroutingPortsRuleGroup,
		Rule: []string{
			string(fam), "daddr", epIP.String(),
			"iifname != {", n.config.IfName, ",", ifNames, `} counter drop comment "DROP DIRECT ACCESS"`,
		},
	})
}
