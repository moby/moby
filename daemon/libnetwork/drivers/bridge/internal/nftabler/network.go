//go:build linux

package nftabler

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/internal/cleanups"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"go.opentelemetry.io/otel"
)

type network struct {
	config  firewaller.NetworkConfig
	cleaner func(ctx context.Context) error
	fw      *Nftabler
}

func (nft *Nftabler) NewNetwork(ctx context.Context, nc firewaller.NetworkConfig) (_ firewaller.Network, retErr error) {
	n := &network{
		fw:     nft,
		config: nc,
	}
	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"bridge": n.config.IfName}))

	var cleaner cleanups.Composite
	defer func() {
		if err := cleaner.Call(ctx); err != nil {
			log.G(ctx).WithError(err).Warn("Failed to clean up nftables rules for network")
		}
	}()

	if nft.cleaner != nil {
		nft.cleaner.DelNetwork(ctx, nc)
	}

	if n.fw.config.IPv4 {
		clean, err := n.configure(ctx, nft.table4, n.config.Config4)
		if err != nil {
			return nil, err
		}
		if clean != nil {
			cleaner.Add(clean)
		}
	}
	if n.fw.config.IPv6 {
		clean, err := n.configure(ctx, nft.table6, n.config.Config6)
		if err != nil {
			return nil, err
		}
		if clean != nil {
			cleaner.Add(clean)
		}
	}

	n.cleaner = cleaner.Release()
	return n, nil
}

func (n *network) configure(ctx context.Context, table nftables.TableRef, conf firewaller.NetworkConfigFam) (func(context.Context) error, error) {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".newNetwork."+string(table.Family()))
	defer span.End()

	if !conf.Prefix.IsValid() {
		return nil, nil
	}

	var cleanup cleanups.Composite
	defer cleanup.Call(ctx)
	var applied bool
	cleanup.Add(func(ctx context.Context) error {
		if applied {
			return nftApply(ctx, table)
		}
		return nil
	})

	// Filter chain

	fwdInChain := table.Chain(ctx, chainFilterFwdIn(n.config.IfName))
	cleanup.Add(func(ctx context.Context) error { return table.DeleteChain(ctx, chainFilterFwdIn(n.config.IfName)) })
	fwdOutChain := table.Chain(ctx, chainFilterFwdOut(n.config.IfName))
	cleanup.Add(func(ctx context.Context) error { return table.DeleteChain(ctx, chainFilterFwdOut(n.config.IfName)) })

	cf, err := table.InterfaceVMap(ctx, filtFwdInVMap).AddElementCf(ctx, n.config.IfName, "jump "+chainFilterFwdIn(n.config.IfName))
	if err != nil {
		return nil, fmt.Errorf("adding filter-forward jump for %s to %q: %w", conf.Prefix, chainFilterFwdIn(n.config.IfName), err)
	}
	cleanup.Add(cf)

	cf, err = table.InterfaceVMap(ctx, filtFwdOutVMap).AddElementCf(ctx, n.config.IfName, "jump "+chainFilterFwdOut(n.config.IfName))
	if err != nil {
		return nil, fmt.Errorf("adding filter-forward jump for %s to %q: %w", conf.Prefix, chainFilterFwdOut(n.config.IfName), err)
	}
	cleanup.Add(cf)

	// NAT chain

	natPostroutingIn := table.Chain(ctx, chainNatPostRtIn(n.config.IfName))
	cleanup.Add(func(ctx context.Context) error { return table.DeleteChain(ctx, chainNatPostRtIn(n.config.IfName)) })
	cf, err = table.InterfaceVMap(ctx, natPostroutingInVMap).AddElementCf(ctx, n.config.IfName, "jump "+chainNatPostRtIn(n.config.IfName))
	if err != nil {
		return nil, fmt.Errorf("adding postrouting ingress jump for %s to %q: %w", conf.Prefix, chainNatPostRtIn(n.config.IfName), err)
	}
	cleanup.Add(cf)

	natPostroutingOut := table.Chain(ctx, chainNatPostRtOut(n.config.IfName))
	cleanup.Add(func(ctx context.Context) error { return table.DeleteChain(ctx, chainNatPostRtOut(n.config.IfName)) })
	cf, err = table.InterfaceVMap(ctx, natPostroutingOutVMap).AddElementCf(ctx, n.config.IfName, "jump "+chainNatPostRtOut(n.config.IfName))
	if err != nil {
		return nil, fmt.Errorf("adding postrouting egress jump for %s to %q: %w", conf.Prefix, chainNatPostRtOut(n.config.IfName), err)
	}
	cleanup.Add(cf)

	// Conntrack

	cf, err = fwdInChain.AppendRuleCf(ctx, initialRuleGroup, "ct state established,related counter accept")
	if err != nil {
		return nil, fmt.Errorf("adding conntrack ingress rule for %q: %w", n.config.IfName, err)
	}
	cleanup.Add(cf)

	cf, err = fwdOutChain.AppendRuleCf(ctx, initialRuleGroup, "ct state established,related counter accept")
	if err != nil {
		return nil, fmt.Errorf("adding conntrack egress rule for %q: %w", n.config.IfName, err)
	}
	cleanup.Add(cf)

	iccVerdict := "accept"
	if !n.config.ICC {
		iccVerdict = "drop"
	}

	if n.config.Internal {
		// Drop anything that's not from this network.
		cf, err = fwdInChain.AppendRuleCf(ctx, initialRuleGroup,
			`iifname != %s counter drop comment "INTERNAL NETWORK INGRESS"`, n.config.IfName)
		if err != nil {
			return nil, fmt.Errorf("adding INTERNAL NETWORK ingress rule for %q: %w", n.config.IfName, err)
		}
		cleanup.Add(cf)

		cf, err = fwdOutChain.AppendRuleCf(ctx, initialRuleGroup,
			`oifname != %s counter drop comment "INTERNAL NETWORK EGRESS"`, n.config.IfName)
		if err != nil {
			return nil, fmt.Errorf("adding INTERNAL NETWORK egress rule for %q: %w", n.config.IfName, err)
		}
		cleanup.Add(cf)

		// Accept or drop Inter-Container Communication.
		cf, err = fwdInChain.AppendRuleCf(ctx, fwdInICCRuleGroup, "counter %s comment ICC", iccVerdict)
		if err != nil {
			return nil, fmt.Errorf("adding ICC ingress rule for %q: %w", n.config.IfName, err)
		}
		cleanup.Add(cf)
	} else {
		// AcceptFwMark
		if n.config.AcceptFwMark != "" {
			fwm, err := nftFwMark(n.config.AcceptFwMark)
			if err != nil {
				return nil, fmt.Errorf("adding fwmark %q for %q: %w", n.config.AcceptFwMark, n.config.IfName, err)
			}
			cf, err = fwdInChain.AppendRuleCf(ctx, fwdInAcceptFwMarkRuleGroup,
				`meta mark %s counter accept comment "ALLOW FW MARK"`, fwm)
			if err != nil {
				return nil, fmt.Errorf("adding ALLOW FW MARK rule for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		}

		// Inter-Container Communication
		cf, err = fwdInChain.AppendRuleCf(ctx, fwdInICCRuleGroup, "iifname == %s counter %s comment ICC",
			n.config.IfName, iccVerdict)
		if err != nil {
			return nil, fmt.Errorf("adding ICC rule for %q: %w", n.config.IfName, err)
		}
		cleanup.Add(cf)

		// Outgoing traffic
		cf, err = fwdOutChain.AppendRuleCf(ctx, initialRuleGroup, "counter accept comment OUTGOING")
		if err != nil {
			return nil, fmt.Errorf("adding OUTGOING rule for %q: %w", n.config.IfName, err)
		}
		cleanup.Add(cf)

		// Incoming traffic
		if conf.Unprotected {
			cf, err = fwdInChain.AppendRuleCf(ctx, fwdInFinalRuleGroup, `counter accept comment "UNPROTECTED"`)
			if err != nil {
				return nil, fmt.Errorf("adding UNPROTECTED for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		} else {
			cf, err = fwdInChain.AppendRuleCf(ctx, fwdInFinalRuleGroup, `counter drop comment "UNPUBLISHED PORT DROP"`)
			if err != nil {
				return nil, fmt.Errorf("adding UNPUBLISHED PORT DROP for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		}

		// ICMP
		if conf.Routed {
			rule := "ip protocol icmp"
			if table.Family() == nftables.IPv6 {
				rule = "meta l4proto ipv6-icmp"
			}
			cf, err = fwdInChain.AppendRuleCf(ctx, initialRuleGroup, rule+" counter accept comment ICMP")
			if err != nil {
				return nil, fmt.Errorf("adding ICMP rule for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		}

		// Masquerade / SNAT - masquerade picks a source IP address based on next-hop, SNAT uses conf.HostIP.
		natPostroutingVerdict := "masquerade"
		natPostroutingComment := "MASQUERADE"
		if conf.HostIP.IsValid() {
			natPostroutingVerdict = "snat to " + conf.HostIP.Unmap().String()
			natPostroutingComment = "SNAT"
		}
		if n.config.Masquerade && !conf.Routed {
			cf, err = natPostroutingOut.AppendRuleCf(ctx, initialRuleGroup, `oifname != %s %s saddr %s counter %s comment "%s"`,
				n.config.IfName, table.Family(), conf.Prefix, natPostroutingVerdict, natPostroutingComment)
			if err != nil {
				return nil, fmt.Errorf("adding NAT rule for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		}
		if n.fw.config.Hairpin {
			// Masquerade/SNAT traffic from localhost.
			cf, err = natPostroutingIn.AppendRuleCf(ctx, initialRuleGroup, `fib saddr type local counter %s comment "%s FROM HOST"`,
				natPostroutingVerdict, natPostroutingComment)
			if err != nil {
				return nil, fmt.Errorf("adding NAT local rule for %q: %w", n.config.IfName, err)
			}
			cleanup.Add(cf)
		}
	}

	ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{
		"bridge": n.config.IfName,
		"family": table.Family(),
	}))
	if err := nftApply(ctx, table); err != nil {
		return nil, fmt.Errorf("adding rules for bridge %s: %w", n.config.IfName, err)
	}
	applied = true

	return cleanup.Release(), nil
}

func (n *network) ReapplyNetworkLevelRules(ctx context.Context) error {
	// A firewalld reload doesn't delete nftables rules, this function is not needed.
	log.G(ctx).Warn("ReapplyNetworkLevelRules is not implemented for nftables")
	return nil
}

func (n *network) DelNetworkLevelRules(ctx context.Context) error {
	if n.cleaner != nil {
		ctx = log.WithLogger(ctx, log.G(ctx).WithFields(log.Fields{"bridge": n.config.IfName}))
		if err := n.cleaner(ctx); err != nil {
			log.G(ctx).WithError(err).Warn("Failed to remove network rules for network")
		}
		n.cleaner = nil
	}
	return nil
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

// nftFwMark takes a string representing a firewall mark with an optional
// "/mask", parses the mark and mask, and returns an nftables expression
// representing the same mask/mark. Numbers are converted to decimal, because
// strings.ParseUint accepts more integer formats than nft.
func nftFwMark(val string) (string, error) {
	markStr, maskStr, haveMask := strings.Cut(val, "/")
	mark, err := strconv.ParseUint(markStr, 0, 32)
	if err != nil {
		return "", fmt.Errorf("invalid firewall mark %q: %w", val, err)
	}
	if haveMask {
		mask, err := strconv.ParseUint(maskStr, 0, 32)
		if err != nil {
			return "", fmt.Errorf("invalid firewall mask %q: %w", val, err)
		}
		return fmt.Sprintf("and %d == %d", mask, mark), nil
	}
	return strconv.FormatUint(mark, 10), nil
}
