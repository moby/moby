//go:build linux

package nftabler

import (
	"context"
	"errors"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/nftables"
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
	table4  nftables.Table
	table6  nftables.Table
}

// NewNftabler creates a new Nftabler instance, initializing the nftables tables.
// Call Close() on the returned Nftabler to release resources when done.
func NewNftabler(ctx context.Context, config firewaller.Config) (*Nftabler, error) {
	nft := &Nftabler{config: config}

	if nft.config.IPv4 {
		var err error
		nft.table4, err = nft.init(ctx, nftables.IPv4)
		if err != nil {
			return nil, err
		}
	}

	if nft.config.IPv6 {
		var err error
		nft.table6, err = nft.init(ctx, nftables.IPv6)
		if err != nil {
			return nil, err
		}
	}

	return nft, nil
}

// Close releases resources held by the Nftabler, the underlying nftables tables
// are not modified or deleted.
func (nft *Nftabler) Close() error {
	return errors.Join(nft.table4.Close(), nft.table6.Close())
}

func (nft *Nftabler) init(ctx context.Context, family nftables.Family) (nftables.Table, error) {
	// Instantiate the table.
	table, err := nftables.NewTable(family, dockerTable)
	if err != nil {
		return table, err
	}
	tm := nftables.Modifier{}

	// Set up the filter forward chain.
	//
	// This base chain only contains two rules that use verdict maps:
	// - if a packet is entering a bridge network, jump to that network's filter-forward ingress chain.
	// - if a packet is leaving a bridge network, jump to that network's filter-forward egress chain.
	//
	// So, packets that aren't related to docker don't need to traverse any per-network filter forward
	// rules - and packets that are entering or leaving docker networks only need to traverse rules
	// related to those networks.
	tm.Create(nftables.BaseChain{
		Name:      forwardChain,
		ChainType: nftables.BaseChainTypeFilter,
		Hook:      nftables.BaseChainHookForward,
		Priority:  nftables.BaseChainPriorityFilter,
	})
	// Instantiate the verdict maps and add the jumps.
	tm.Create(nftables.VMap{
		Name:        filtFwdInVMap,
		ElementType: nftables.NftTypeIfname,
	})
	tm.Create(nftables.Rule{
		Chain: forwardChain,
		Group: initialRuleGroup,
		Rule:  []string{"oifname vmap @", filtFwdInVMap},
	})

	tm.Create(nftables.VMap{
		Name:        filtFwdOutVMap,
		ElementType: nftables.NftTypeIfname,
	})
	tm.Create(nftables.Rule{
		Chain: forwardChain,
		Group: initialRuleGroup,
		Rule:  []string{"iifname vmap @", filtFwdOutVMap},
	})

	// Set up the NAT postrouting base chain.
	//
	// Like the filter-forward chain, its only rules are jumps to network-specific ingress and egress chains.
	tm.Create(nftables.BaseChain{
		Name:      postroutingChain,
		ChainType: nftables.BaseChainTypeNAT,
		Hook:      nftables.BaseChainHookPostrouting,
		Priority:  nftables.BaseChainPrioritySrcNAT,
	})

	tm.Create(nftables.VMap{
		Name:        natPostroutingOutVMap,
		ElementType: nftables.NftTypeIfname,
	})
	tm.Create(nftables.Rule{
		Chain: postroutingChain,
		Group: initialRuleGroup,
		Rule:  []string{"iifname vmap @", natPostroutingOutVMap},
	})

	tm.Create(nftables.VMap{
		Name:        natPostroutingInVMap,
		ElementType: nftables.NftTypeIfname,
	})
	tm.Create(nftables.Rule{
		Chain: postroutingChain,
		Group: initialRuleGroup,
		Rule:  []string{"oifname vmap @", natPostroutingInVMap},
	})

	// Instantiate natChain, for the NAT prerouting and output base chains to jump to.
	tm.Create(nftables.Chain{
		Name: natChain,
	})

	// Set up the NAT prerouting base chain.
	tm.Create(nftables.BaseChain{
		Name:      preroutingChain,
		ChainType: nftables.BaseChainTypeNAT,
		Hook:      nftables.BaseChainHookPrerouting,
		Priority:  nftables.BaseChainPriorityDstNAT,
	})
	tm.Create(nftables.Rule{
		Chain: preroutingChain,
		Group: initialRuleGroup,
		Rule:  []string{"fib daddr type local counter jump", natChain},
	})

	// Set up the NAT output base chain
	tm.Create(nftables.BaseChain{
		Name:      outputChain,
		ChainType: nftables.BaseChainTypeNAT,
		Hook:      nftables.BaseChainHookOutput,
		Priority:  nftables.BaseChainPriorityDstNAT,
	})

	// For output, don't jump to the NAT chain if hairpin is enabled (no userland proxy).
	var skipLoopback string
	if !nft.config.Hairpin {
		if family == nftables.IPv4 {
			skipLoopback = "ip daddr != 127.0.0.1/8 "
		} else {
			skipLoopback = "ip6 daddr != ::1 "
		}
	}
	tm.Create(nftables.Rule{
		Chain: outputChain,
		Group: initialRuleGroup,
		Rule:  []string{skipLoopback, "fib daddr type local counter jump", natChain},
	})

	// Set up the raw prerouting base chain
	tm.Create(nftables.BaseChain{
		Name:      rawPreroutingChain,
		ChainType: nftables.BaseChainTypeFilter,
		Hook:      nftables.BaseChainHookPrerouting,
		Priority:  nftables.BaseChainPriorityRaw,
	})

	// WSL2 does not (currently) support Windows<->Linux communication via ::1.
	if !nft.config.Hairpin && nft.config.WSL2Mirrored && table.Family() == nftables.IPv4 {
		mirroredWSL2Workaround(&tm)
	}

	if err := table.Apply(ctx, tm); err != nil {
		if family == nftables.IPv4 {
			return nftables.Table{}, err
		}
		// Perhaps the kernel has no IPv6 support. It won't be possible to create IPv6
		// networks without enabling ip6_tables in the kernel, or disabling ip6tables in
		// the daemon config. But, allow the daemon to start because IPv4 will work. So,
		// log the problem, and continue.
		log.G(ctx).WithError(err).Warn("ip6tables is enabled, but cannot set up IPv6 nftables table")
		return nftables.Table{}, nil
	}
	return table, nil
}
