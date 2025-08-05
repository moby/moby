//go:build linux

package nftabler

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
	"go.opentelemetry.io/otel"
)

// Prefix for OTEL span names.
const spanPrefix = "libnetwork.drivers.bridge.nftabler"

const (
	dockerTable           = "docker-bridges"
	forwardChain          = "filter-FORWARD"
	postroutingChain      = "nat-POSTROUTING"
	preroutingChain       = "nat-PREROUTING"
	outputChain           = "nat-OUTPUT"
	natChain              = "nat-prerouting-and-output"
	rawPreroutingChain    = "raw-PREROUTING"
	filtFwdInVMap         = "filter-forward-in-jumps"
	filtFwdOutVMap        = "filter-forward-out-jumps"
	natPostroutingOutVMap = "nat-postrouting-out-jumps"
	natPostroutingInVMap  = "nat-postrouting-in-jumps"
)

const (
	initialRuleGroup nftables.RuleGroup = iota
)

const (
	fwdInAcceptFwMarkRuleGroup = iota + initialRuleGroup + 1
	fwdInLegacyLinksRuleGroup
	fwdInICCRuleGroup
	fwdInPortsRuleGroup
	fwdInFinalRuleGroup
)

const (
	rawPreroutingPortsRuleGroup = iota + initialRuleGroup + 1
)

type Nftabler struct {
	config  firewaller.Config
	cleaner firewaller.FirewallCleaner
	table4  nftables.TableRef
	table6  nftables.TableRef
}

func NewNftabler(ctx context.Context, config firewaller.Config) (*Nftabler, error) {
	nft := &Nftabler{config: config}

	if nft.config.IPv4 {
		var err error
		nft.table4, err = nft.init(ctx, nftables.IPv4)
		if err != nil {
			return nil, err
		}
		if err := nftApply(ctx, nft.table4); err != nil {
			return nil, fmt.Errorf("IPv4 initialisation: %w", err)
		}
	}

	if nft.config.IPv6 {
		var err error
		nft.table6, err = nft.init(ctx, nftables.IPv6)
		if err != nil {
			return nil, err
		}

		if err := nftApply(ctx, nft.table6); err != nil {
			// Perhaps the kernel has no IPv6 support. It won't be possible to create IPv6
			// networks without enabling ip6_tables in the kernel, or disabling ip6tables in
			// the daemon config. But, allow the daemon to start because IPv4 will work. So,
			// log the problem, and continue.
			log.G(ctx).WithError(err).Warn("ip6tables is enabled, but cannot set up IPv6 nftables table")
		}
	}

	return nft, nil
}

// init creates the bridge driver's nftables table for IPv4 or IPv6.
func (nft *Nftabler) init(ctx context.Context, family nftables.Family) (nftables.TableRef, error) {
	// Instantiate the table.
	table, err := nftables.NewTable(family, dockerTable)
	if err != nil {
		return table, err
	}

	// Set up the filter forward chain.
	//
	// This base chain only contains two rules that use verdict maps:
	// - if a packet is entering a bridge network, jump to that network's filter-forward ingress chain.
	// - if a packet is leaving a bridge network, jump to that network's filter-forward egress chain.
	//
	// So, packets that aren't related to docker don't need to traverse any per-network filter forward
	// rules - and packets that are entering or leaving docker networks only need to traverse rules
	// related to those networks.
	fwdChain, err := table.BaseChain(ctx, forwardChain,
		nftables.BaseChainTypeFilter,
		nftables.BaseChainHookForward,
		nftables.BaseChainPriorityFilter)
	if err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}
	// Instantiate the verdict maps and add the jumps.
	_ = table.InterfaceVMap(ctx, filtFwdInVMap)
	if err := fwdChain.AppendRule(ctx, initialRuleGroup, "oifname vmap @"+filtFwdInVMap); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}
	_ = table.InterfaceVMap(ctx, filtFwdOutVMap)
	if err := fwdChain.AppendRule(ctx, initialRuleGroup, "iifname vmap @"+filtFwdOutVMap); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}

	// Set up the NAT postrouting base chain.
	//
	// Like the filter-forward chain, its only rules are jumps to network-specific ingress and egress chains.
	natPostRtChain, err := table.BaseChain(ctx, postroutingChain,
		nftables.BaseChainTypeNAT,
		nftables.BaseChainHookPostrouting,
		nftables.BaseChainPrioritySrcNAT)
	if err != nil {
		return nftables.TableRef{}, err
	}
	_ = table.InterfaceVMap(ctx, natPostroutingOutVMap)
	if err := natPostRtChain.AppendRule(ctx, initialRuleGroup, "iifname vmap @"+natPostroutingOutVMap); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}
	_ = table.InterfaceVMap(ctx, natPostroutingInVMap)
	if err := natPostRtChain.AppendRule(ctx, initialRuleGroup, "oifname vmap @"+natPostroutingInVMap); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}

	// Instantiate natChain, for the NAT prerouting and output base chains to jump to.
	_ = table.Chain(ctx, natChain)

	// Set up the NAT prerouting base chain.
	natPreRtChain, err := table.BaseChain(ctx, preroutingChain,
		nftables.BaseChainTypeNAT,
		nftables.BaseChainHookPrerouting,
		nftables.BaseChainPriorityDstNAT)
	if err != nil {
		return nftables.TableRef{}, err
	}
	if err := natPreRtChain.AppendRule(ctx, initialRuleGroup, "fib daddr type local counter jump "+natChain); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}

	// Set up the NAT output base chain
	natOutputChain, err := table.BaseChain(ctx, outputChain,
		nftables.BaseChainTypeNAT,
		nftables.BaseChainHookOutput,
		nftables.BaseChainPriorityDstNAT)
	if err != nil {
		return nftables.TableRef{}, err
	}
	// For output, don't jump to the NAT chain if hairpin is enabled (no userland proxy).
	var skipLoopback string
	if !nft.config.Hairpin {
		if family == nftables.IPv4 {
			skipLoopback = "ip daddr != 127.0.0.1/8 "
		} else {
			skipLoopback = "ip6 daddr != ::1 "
		}
	}
	if err := natOutputChain.AppendRule(ctx, initialRuleGroup, skipLoopback+"fib daddr type local counter jump "+natChain); err != nil {
		return nftables.TableRef{}, fmt.Errorf("initialising nftables: %w", err)
	}

	// Set up the raw prerouting base chain
	if _, err := table.BaseChain(ctx, rawPreroutingChain,
		nftables.BaseChainTypeFilter,
		nftables.BaseChainHookPrerouting,
		nftables.BaseChainPriorityRaw); err != nil {
		return nftables.TableRef{}, err
	}

	if !nft.config.Hairpin && nft.config.WSL2Mirrored {
		if err := mirroredWSL2Workaround(ctx, table); err != nil {
			return nftables.TableRef{}, err
		}
	}

	return table, nil
}

func nftApply(ctx context.Context, table nftables.TableRef) error {
	ctx, span := otel.Tracer("").Start(ctx, spanPrefix+".nftApply."+string(table.Family()))
	defer span.End()
	if err := table.Apply(ctx); err != nil {
		return fmt.Errorf("applying nftables rules: %w", err)
	}
	return nil
}
