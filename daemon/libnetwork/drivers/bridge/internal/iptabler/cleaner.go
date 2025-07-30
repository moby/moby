//go:build linux

package iptabler

import (
	"context"
	"net/netip"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/docker/docker/daemon/libnetwork/iptables"
	"github.com/docker/docker/daemon/libnetwork/types"
)

type iptablesCleaner struct {
	config             firewaller.Config
	filterForwardDrop4 bool
	filterForwardDrop6 bool
}

// NewCleaner checks for iptables rules left behind by an old daemon that was using
// the iptabler.
//
// If there are old rules present, it deletes as much as possible straight away
// (user-defined chains and jumps from the built-in chains).
//
// But, it can't delete network or port-specific rules from built-in chains
// without flushing those chains which would be very antisocial - because, at
// this stage, interface names, addresses, etc. are unknown. So, also return
// FirewallCleaner that the new Firewaller can use to delete those rules while
// it's setting up those networks/ports (probably on replay from persistent
// storage).
//
// If there are no old rules to clean up, return nil.
func NewCleaner(ctx context.Context, config firewaller.Config) firewaller.FirewallCleaner {
	clean := func(ipv iptables.IPVersion, enabled bool) (ffDrop bool, cleaned bool) {
		if !enabled {
			return false, false
		}
		t := iptables.GetIptable(ipv)
		// Since 28.0, the jump in the filter-FORWARD chain is DOCKER-FORWARD.
		// In earlier releases, there was a jump to DOCKER-ISOLATION-STAGE-1.
		if !t.Exists("filter", "FORWARD", "-j", DockerForwardChain) &&
			!t.Exists("filter", "FORWARD", "-j", isolationChain1) {
			return false, false
		}
		log.G(ctx).WithField("ipv", ipv).Info("Cleaning iptables")
		_ = t.DeleteJumpRule(iptables.Filter, "FORWARD", DockerForwardChain)
		_ = deleteLegacyTopLevelRules(ctx, t, ipv)
		removeIPChains(ctx, ipv)
		return t.HasPolicy("filter", "FORWARD", iptables.Drop), true
	}
	ffDrop4, cleaned4 := clean(iptables.IPv4, config.IPv4)
	ffDrop6, cleaned6 := clean(iptables.IPv6, config.IPv6)
	if !cleaned4 && !cleaned6 {
		return nil
	}
	return &iptablesCleaner{
		config:             config,
		filterForwardDrop4: ffDrop4,
		filterForwardDrop6: ffDrop6,
	}
}

func (ic iptablesCleaner) DelNetwork(ctx context.Context, nc firewaller.NetworkConfig) {
	if nc.Internal {
		return
	}
	n := network{
		config: nc,
		ipt:    &iptabler{config: ic.config},
	}
	if ic.config.IPv4 && nc.Config4.Prefix.IsValid() {
		_ = deleteLegacyFilterRules(iptables.IPv4, nc.IfName)
		_ = n.setupNonInternalNetworkRules(ctx, iptables.IPv4, nc.Config4, false)
	}
	if ic.config.IPv6 && nc.Config6.Prefix.IsValid() {
		_ = deleteLegacyFilterRules(iptables.IPv6, nc.IfName)
		_ = n.setupNonInternalNetworkRules(ctx, iptables.IPv6, nc.Config6, false)
	}
}

func (ic iptablesCleaner) DelEndpoint(ctx context.Context, nc firewaller.NetworkConfig, epIPv4, epIPv6 netip.Addr) {
	n := network{
		config: nc,
		ipt:    &iptabler{config: ic.config},
	}
	if n.ipt.config.IPv4 && epIPv4.IsValid() {
		_ = n.filterDirectAccess(ctx, iptables.IPv4, n.config.Config4, epIPv4, false)
	}
	if n.ipt.config.IPv6 && epIPv6.IsValid() {
		_ = n.filterDirectAccess(ctx, iptables.IPv6, n.config.Config6, epIPv6, false)
	}
}

func (ic iptablesCleaner) DelPorts(ctx context.Context, nc firewaller.NetworkConfig, pbs []types.PortBinding) {
	n := network{
		config: nc,
		ipt:    &iptabler{config: ic.config},
	}
	_ = n.DelPorts(ctx, pbs)
}

func (ic iptablesCleaner) HadFilterForwardDrop(ipv firewaller.IPVersion) bool {
	if ipv == firewaller.IPv4 {
		return ic.filterForwardDrop4
	}
	return ic.filterForwardDrop4
}

func (ic iptablesCleaner) SetFilterForwardAccept(ipv firewaller.IPVersion) error {
	iptv := iptables.IPv4
	if ipv == firewaller.IPv6 {
		iptv = iptables.IPv6
	}
	return iptables.GetIptable(iptv).SetDefaultPolicy("filter", "FORWARD", iptables.Accept)
}
