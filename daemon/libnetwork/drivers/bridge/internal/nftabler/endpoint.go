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
		if err := n.filterDirectAccess(ctx, n.fw.table4, n.config.Config4, epIPv4, enable); err != nil {
			return err
		}
		if err := nftApply(ctx, n.fw.table4); err != nil {
			return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
		}
	}
	if n.fw.config.IPv6 && epIPv6.IsValid() {
		if err := n.filterDirectAccess(ctx, n.fw.table6, n.config.Config6, epIPv6, enable); err != nil {
			return err
		}
		if err := nftApply(ctx, n.fw.table6); err != nil {
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
// container are allowed - the host always has direct access to its own containers
// (it doesn't need to use the port mapped to its own addresses, although it can).
//
// "Trusted interfaces" are treated in the same way as the bridge itself.
func (n *network) filterDirectAccess(ctx context.Context, table nftables.TableRef, conf firewaller.NetworkConfigFam, epIP netip.Addr, enable bool) error {
	if n.config.Internal || conf.Unprotected || conf.Routed || n.fw.config.AllowDirectRouting {
		return nil
	}
	updater := table.ChainUpdateFunc(ctx, rawPreroutingChain, enable)
	ifNames := strings.Join(n.config.TrustedHostInterfaces, ", ")
	return updater(ctx, rawPreroutingPortsRuleGroup,
		`%s daddr %s iifname != { %s, %s } counter drop comment "DROP DIRECT ACCESS"`,
		table.Family(), epIP, n.config.IfName, ifNames)
}
