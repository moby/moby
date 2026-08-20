//go:build linux

package iptabler

import (
	"context"
	"fmt"
	"time"

	"github.com/containerd/log"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge/internal/firewaller"
	"github.com/moby/moby/v2/daemon/libnetwork/internal/modprobe"
	"github.com/moby/moby/v2/daemon/libnetwork/iptables"
)

const (
	// dockerChain: DOCKER iptable chain name
	dockerChain = "DOCKER"
	// DockerForwardChain contains Docker's filter-FORWARD rules.
	//
	// FIXME(robmry) - only exported because it's used to set up the jump to swarm's DOCKER-INGRESS chain.
	DockerForwardChain  = "DOCKER-FORWARD"
	dockerBridgeChain   = "DOCKER-BRIDGE"
	dockerCTChain       = "DOCKER-CT"
	dockerInternalChain = "DOCKER-INTERNAL"

	// These INC (inter-network communication) chains are no longer needed, packets
	// sent to unpublished ports in other networks are now dropped by rules in the DOCKER
	// chain. Packets sent directly to published ports in a different network don't need
	// to be dropped:
	// - containers in other networks have access via the host's address, and
	// - it was surprising that a container in a gwmode=nat network couldn't talk to a
	//   published port in a gwmode=routed network, but anything outside a bridge
	//   network could.
	isolationChain1 = "DOCKER-ISOLATION-STAGE-1"
	isolationChain2 = "DOCKER-ISOLATION-STAGE-2"
)

type Iptabler struct {
	config firewaller.Config
}

func NewIptabler(ctx context.Context, config firewaller.Config) (*Iptabler, error) {
	ipt := &Iptabler{config: config}

	if ipt.config.IPv4 {
		removeIPChains(ctx, iptables.IPv4)

		if err := setupIPChains(ctx, iptables.IPv4, ipt.config); err != nil {
			return nil, err
		}

		// On firewall reload, re-create chains from scratch. removeIPChains
		// uses the native iptables binary (bypassing firewalld's passthrough
		// store) so these flush commands are not accumulated by firewalld and
		// replayed on future reloads, which would flush per-network rules that
		// Docker re-establishes after the reload.
		iptables.OnReloaded(func() {
			log.G(ctx).Debugf("Recreating iptables chains on firewall reload")
			reloadIPChains(ctx, iptables.IPv4, ipt.config, "iptables")
		})
	}

	if ipt.config.IPv6 {
		if err := modprobe.LoadModules(ctx, func() error {
			iptable := iptables.GetIptable(iptables.IPv6)
			_, err := iptable.Raw("-t", "filter", "-n", "-L", "FORWARD")
			return err
		}, "ip6_tables"); err != nil {
			log.G(ctx).WithError(err).Debug("Loading ip6_tables")
		}

		removeIPChains(ctx, iptables.IPv6)

		err := setupIPChains(ctx, iptables.IPv6, ipt.config)
		if err != nil {
			// If the chains couldn't be set up, it's probably because the kernel has no IPv6
			// support, or it doesn't have module ip6_tables loaded. It won't be possible to
			// create IPv6 networks without enabling ip6_tables in the kernel, or disabling
			// ip6tables in the daemon config. But, allow the daemon to start because IPv4
			// will work. So, log the problem, and continue.
			log.G(ctx).WithError(err).Warn("ip6tables is enabled, but cannot set up ip6tables chains")
		} else {
			// Same as the IPv4 case: re-create chains on reload using
			// native iptables calls in removeIPChains to avoid storing
			// flush commands in firewalld's passthrough history.
			iptables.OnReloaded(func() {
				log.G(ctx).Debugf("Recreating ip6tables chains on firewall reload")
				reloadIPChains(ctx, iptables.IPv6, ipt.config, "ip6tables")
			})
		}
	}

	return ipt, nil
}

// FilterForwardDrop sets the default policy of the FORWARD chain in the filter table to DROP.
func (ipt *Iptabler) FilterForwardDrop(ctx context.Context, ipv firewaller.IPVersion) error {
	var iptv iptables.IPVersion
	switch ipv {
	case firewaller.IPv4:
		iptv = iptables.IPv4
	case firewaller.IPv6:
		iptv = iptables.IPv6
	default:
		return fmt.Errorf("unknown IP version %v", ipv)
	}
	iptable := iptables.GetIptable(iptv)
	if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
		return err
	}
	iptables.OnReloaded(func() {
		log.G(ctx).WithFields(log.Fields{"ipv": ipv}).Debug("Setting the default DROP policy on firewall reload")
		if err := iptable.SetDefaultPolicy(iptables.Filter, "FORWARD", iptables.Drop); err != nil {
			log.G(ctx).WithFields(log.Fields{
				"error": err,
				"ipv":   ipv,
			}).Warn("Failed to set the default DROP policy on firewall reload")
		}
	})
	return nil
}

// reloadIPChains removes and re-creates Docker iptables chains after a firewall
// reload. removeIPChains uses the native iptables binary (bypassing firewalld's
// passthrough store) to avoid accumulating flush commands that firewalld would
// replay on future reloads. It retries on failure to handle the race where
// firewalld's concurrent chain cleanup deletes chains that setupIPChains just
// created; retrying after a short delay (by which time the cleanup has
// finished) restores a clean state.
func reloadIPChains(ctx context.Context, version iptables.IPVersion, cfg firewaller.Config, name string) {
	const (
		maxAttempts = 3
		retryDelay  = 100 * time.Millisecond
	)
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		removeIPChains(ctx, version)
		if err := setupIPChains(ctx, version, cfg); err == nil {
			return
		} else if attempt < maxAttempts {
			log.G(ctx).WithError(err).Debugf(
				"%s chain setup attempt %d/%d failed on reload, retrying",
				name, attempt, maxAttempts)
			time.Sleep(retryDelay)
		} else {
			log.G(ctx).WithError(err).Errorf("Error reloading %s chains", name)
		}
	}
}

func setupIPChains(ctx context.Context, version iptables.IPVersion, iptCfg firewaller.Config) error {
	iptable := iptables.GetIptable(version)

	// Note: no deferred cleanup on failure here. When called from
	// reloadIPChains, cleanup is handled by the removeIPChains call at the
	// start of each retry attempt. Cleaning up inside setupIPChains would
	// remove all Docker chains if every retry fails, leaving the system with
	// no chains at all — worse than a partial setup, which at least allows
	// network creation to succeed.
	_, err := iptable.NewChain(dockerChain, iptables.Nat)
	if err != nil {
		return fmt.Errorf("failed to create NAT chain %s: %v", dockerChain, err)
	}

	_, err = iptable.NewChain(dockerChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerChain, err)
	}

	_, err = iptable.NewChain(DockerForwardChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", DockerForwardChain, err)
	}

	_, err = iptable.NewChain(dockerBridgeChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerBridgeChain, err)
	}

	_, err = iptable.NewChain(dockerCTChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerCTChain, err)
	}

	_, err = iptable.NewChain(dockerInternalChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER internal chain: %v", err)
	}

	if err := addNATJumpRules(version, iptCfg.Hairpin, true); err != nil {
		return fmt.Errorf("failed to add jump rules to %s NAT table: %w", version, err)
	}

	// Make sure the filter-FORWARD chain has rules to accept related packets and
	// jump to the isolation and docker chains. (Re-)insert at the top of the table,
	// in reverse order.
	if err := iptable.EnsureJumpRule(iptables.Filter, "FORWARD", DockerForwardChain); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(iptables.Filter, DockerForwardChain, dockerBridgeChain); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(iptables.Filter, DockerForwardChain, dockerInternalChain); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(iptables.Filter, DockerForwardChain, dockerCTChain); err != nil {
		return err
	}

	if err := mirroredWSL2Workaround(version, !iptCfg.Hairpin && iptCfg.WSL2Mirrored); err != nil {
		return err
	}

	return deleteLegacyTopLevelRules(ctx, iptable, version)
}

// Delete rules that may have been added to the FORWARD chain by moby 28.0.0 or earlier.
func deleteLegacyTopLevelRules(ctx context.Context, iptable *iptables.IPTable, version iptables.IPVersion) error {
	ipsetName := "docker-ext-bridges-v4"
	if version == iptables.IPv6 {
		ipsetName = "docker-ext-bridges-v6"
	}
	if err := iptable.DeleteJumpRule(iptables.Filter, "FORWARD", dockerChain,
		"-m", "set", "--match-set", ipsetName, "dst"); err != nil {
		log.G(ctx).WithFields(log.Fields{"error": err, "set": ipsetName}).Debug(
			"deleting legacy ipset dest match rule")
	}
	if err := iptable.DeleteJumpRule(iptables.Filter, "FORWARD", isolationChain1); err != nil {
		return err
	}
	if err := iptable.DeleteJumpRule(iptables.Filter, "FORWARD", "ACCEPT",
		"-m", "set", "--match-set", ipsetName, "dst",
		"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED",
	); err != nil {
		log.G(ctx).WithFields(log.Fields{"error": err, "set": ipsetName}).Debug(
			"deleting legacy ipset conntrack rule")
	}

	return nil
}

func programChainRule(rule iptables.Rule, ruleDescr string, insert bool) error {
	operation := "disable"
	fn := rule.Delete
	if insert {
		operation = "enable"
		fn = rule.Insert
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Unable to %s %s rule: %w", operation, ruleDescr, err)
	}
	return nil
}

func appendOrDelChainRule(rule iptables.Rule, ruleDescr string, shouldAppend bool) error {
	operation := "disable"
	fn := rule.Delete
	if shouldAppend {
		operation = "enable"
		fn = rule.Append
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Unable to %s %s rule: %w", operation, ruleDescr, err)
	}
	return nil
}
