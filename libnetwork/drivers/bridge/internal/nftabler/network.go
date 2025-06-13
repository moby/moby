//go:build linux

package nftabler

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/libnetwork/internal/nftables"
	"go.opentelemetry.io/otel"
)

type network struct {
	config firewaller.NetworkConfig
	fw     *nftabler
}

func (nft *nftabler) NewNetwork(ctx context.Context, nc firewaller.NetworkConfig) (_ firewaller.Network, retErr error) {
	n := &network{
		fw:     nft,
		config: nc,
	}
	defer func() {
		if retErr != nil {
			if err := n.DelNetworkLevelRules(ctx); err != nil {
				log.G(ctx).WithError(err).Error("Ignoring cleanup error")
			}
		}
	}()

	if n.fw.config.IPv4 {
		if err := n.configure(ctx, nft.table4, n.config.Config4); err != nil {
			return nil, err
		}
	}
	if n.fw.config.IPv6 {
		if err := n.configure(ctx, nft.table6, n.config.Config6); err != nil {
			return nil, err
		}
	}
	return n, nil
}

func (n *network) configure(ctx context.Context, table nftables.TableRef, conf firewaller.NetworkConfigFam) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".newNetwork."+string(table.Family()))
	defer span.End()

	if !conf.Prefix.IsValid() {
		return nil
	}

	// Filter chain
	fwdInChain := table.Chain(chainFilterFwdIn(n.config.IfName))
	fwdOutChain := table.Chain(chainFilterFwdOut(n.config.IfName))
	if err := table.InterfaceVMap(filtFwdInVMap).AddElement(n.config.IfName, "jump "+chainFilterFwdIn(n.config.IfName)); err != nil {
		return fmt.Errorf("adding filter-forward jump for %s to %q: %w", conf.Prefix, chainFilterFwdIn(n.config.IfName), err)
	}
	if err := table.InterfaceVMap(filtFwdOutVMap).AddElement(n.config.IfName, "jump "+chainFilterFwdOut(n.config.IfName)); err != nil {
		return fmt.Errorf("adding filter-forward jump for %s to %q: %w", conf.Prefix, chainFilterFwdOut(n.config.IfName), err)
	}

	// NAT chain
	natPostroutingIn := table.Chain(chainNatPostRtIn(n.config.IfName))
	if err := table.InterfaceVMap(natPostroutingInVMap).AddElement(n.config.IfName, "jump "+chainNatPostRtIn(n.config.IfName)); err != nil {
		return fmt.Errorf("adding postrouting ingress jump for %s to %q: %w", conf.Prefix, chainNatPostRtIn(n.config.IfName), err)
	}
	natPostroutingOut := table.Chain(chainNatPostRtOut(n.config.IfName))
	if err := table.InterfaceVMap(natPostroutingOutVMap).AddElement(n.config.IfName, "jump "+chainNatPostRtOut(n.config.IfName)); err != nil {
		return fmt.Errorf("adding postrouting egress jump for %s to %q: %w", conf.Prefix, chainNatPostRtOut(n.config.IfName), err)
	}

	// Conntrack
	if err := fwdInChain.AppendRule(initialRuleGroup, "ct state established,related counter accept"); err != nil {
		return fmt.Errorf("adding conntrack ingress rule for %q: %w", n.config.IfName, err)
	}
	if err := fwdOutChain.AppendRule(initialRuleGroup, "ct state established,related counter accept"); err != nil {
		return fmt.Errorf("adding conntrack egress rule for %q: %w", n.config.IfName, err)
	}

	iccVerdict := "accept"
	if !n.config.ICC {
		iccVerdict = "drop"
	}

	if n.config.Internal {
		// Drop anything that's not from this network.
		if err := fwdInChain.AppendRule(initialRuleGroup,
			`iifname != %s counter drop comment "INTERNAL NETWORK INGRESS"`, n.config.IfName); err != nil {
			return fmt.Errorf("adding INTERNAL NETWORK ingress rule for %q: %w", n.config.IfName, err)
		}
		if err := fwdOutChain.AppendRule(initialRuleGroup,
			`oifname != %s counter drop comment "INTERNAL NETWORK EGRESS"`, n.config.IfName); err != nil {
			return fmt.Errorf("adding INTERNAL NETWORK egress rule for %q: %w", n.config.IfName, err)
		}
		// Accept or drop Inter-Container Communication.
		if err := fwdInChain.AppendRule(fwdInICCRuleGroup, "counter %s comment ICC", iccVerdict); err != nil {
			return fmt.Errorf("adding ICC ingress rule for %q: %w", n.config.IfName, err)
		}
	} else {
		// Inter-Container Communication
		if err := fwdInChain.AppendRule(fwdInICCRuleGroup, "iifname == %s counter %s comment ICC",
			n.config.IfName, iccVerdict); err != nil {
			return fmt.Errorf("adding ICC rule for %q: %w", n.config.IfName, err)
		}

		// Outgoing traffic
		if err := fwdOutChain.AppendRule(initialRuleGroup, "counter accept comment OUTGOING"); err != nil {
			return fmt.Errorf("adding OUTGOING rule for %q: %w", n.config.IfName, err)
		}

		// Incoming traffic
		if conf.Unprotected {
			if err := fwdInChain.AppendRule(fwdInFinalRuleGroup, `counter accept comment "UNPROTECTED"`); err != nil {
				return fmt.Errorf("adding UNPROTECTED for %q: %w", n.config.IfName, err)
			}
		} else {
			if err := fwdInChain.AppendRule(fwdInFinalRuleGroup, `counter drop comment "UNPUBLISHED PORT DROP"`); err != nil {
				return fmt.Errorf("adding UNPUBLISHED PORT DROP for %q: %w", n.config.IfName, err)
			}
		}

		// ICMP
		if conf.Routed {
			rule := "ip protocol icmp"
			if table.Family() == nftables.IPv6 {
				rule = "meta l4proto ipv6-icmp"
			}
			if err := fwdInChain.AppendRule(initialRuleGroup, rule+" counter accept comment ICMP"); err != nil {
				return fmt.Errorf("adding ICMP rule for %q: %w", n.config.IfName, err)
			}
		}

		// Masquerade / SNAT - masquerade picks a source IP address based on next-hop, SNAT uses conf.HostIP.
		natPostroutingVerdict := "masquerade"
		natPostroutingComment := "MASQUERADE"
		if conf.HostIP.IsValid() {
			natPostroutingVerdict = "snat to " + conf.HostIP.Unmap().String()
			natPostroutingComment = "SNAT"
		}
		if n.config.Masquerade && !conf.Routed {
			if err := natPostroutingOut.AppendRule(initialRuleGroup, `oifname != %s %s saddr %s counter %s comment "%s"`,
				n.config.IfName, table.Family(), conf.Prefix, natPostroutingVerdict, natPostroutingComment); err != nil {
				return fmt.Errorf("adding NAT rule for %q: %w", n.config.IfName, err)
			}
		}
		if n.fw.config.Hairpin {
			// Masquerade/SNAT traffic from localhost.
			if err := natPostroutingIn.AppendRule(initialRuleGroup, `fib saddr type local counter %s comment "%s FROM HOST"`,
				natPostroutingVerdict, natPostroutingComment); err != nil {
				return fmt.Errorf("adding NAT local rule for %q: %w", n.config.IfName, err)
			}
		}
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"bridge": n.config.IfName,
		"family": table.Family(),
	}))
	if err := nftApply(ctx, table); err != nil {
		return fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
	}

	return nil
}

func (n *network) ReapplyNetworkLevelRules(ctx context.Context) error {
	log.G(ctx).Warn("ReapplyNetworkLevelRules is not implemented for nftables")
	return nil
}

func (n *network) DelNetworkLevelRules(ctx context.Context) error {
	var errs []error
	if n.fw.config.IPv4 && n.config.Config4.Prefix.IsValid() {
		n.cleanupFam(ctx, n.fw.table4, n.config.Config4)
	}
	if n.fw.config.IPv6 && n.config.Config6.Prefix.IsValid() {
		n.cleanupFam(ctx, n.fw.table6, n.config.Config6)
	}
	return errors.Join(errs...)
}

func (n *network) cleanupFam(ctx context.Context, table nftables.TableRef, conf firewaller.NetworkConfigFam) {
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"bridge": n.config.IfName,
		"family": table.Family(),
	}))

	// Filter forward chain
	if err := table.InterfaceVMap(filtFwdInVMap).DeleteElement(n.config.IfName); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting filter-forward dest jump")
	}
	if err := table.InterfaceVMap(filtFwdOutVMap).DeleteElement(n.config.IfName); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting filter-forward dest jump")
	}
	if err := table.DeleteChain(chainFilterFwdIn(n.config.IfName)); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting chain")
	}
	if err := table.DeleteChain(chainFilterFwdOut(n.config.IfName)); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting chain")
	}

	// NAT postrouting chain
	if err := table.InterfaceVMap(natPostroutingOutVMap).DeleteElement(n.config.IfName); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting nat-postrouting out jump")
	}
	if err := table.InterfaceVMap(natPostroutingInVMap).DeleteElement(n.config.IfName); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting nat-postrouting in jump")
	}
	if err := table.DeleteChain(chainNatPostRtOut(n.config.IfName)); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting postrouting out chain")
	}
	if err := table.DeleteChain(chainNatPostRtIn(n.config.IfName)); err != nil {
		log.G(ctx).WithError(err).Debug("Deleting postrouting in chain")
	}

	if err := nftApply(ctx, table); err != nil {
		log.G(ctx).WithError(err).Warn("Failed to remove nftables rules")
	}
}

func chainFilterFwdIn(ifName string) string {
	return "filter-forward-in__" + ifName
}

func chainFilterFwdOut(ifName string) string {
	return "filter-forward-out__" + ifName
}

func chainNatPostRtOut(ifName string) string {
	return "nat-postrouting-out__" + ifName
}

func chainNatPostRtIn(ifName string) string {
	return "nat-postrouting-in__" + ifName
}
