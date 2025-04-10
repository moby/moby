//go:build linux

package iptabler

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/internal/modprobe"
	"github.com/docker/docker/libnetwork/iptables"
)

const (
	// dockerChain: DOCKER iptable chain name
	dockerChain = "DOCKER"
	// DockerForwardChain contains Docker's filter-FORWARD rules.
	//
	// FIXME(robmry) - only exported because it's used to set up the jump to swarm's DOCKER-INGRESS chain.
	DockerForwardChain = "DOCKER-FORWARD"
	dockerBridgeChain  = "DOCKER-BRIDGE"
	dockerCTChain      = "DOCKER-CT"

	// Isolation between bridge networks is achieved in two stages by means
	// of the following two chains in the filter table. The first chain matches
	// on the source interface being a bridge network's bridge and the
	// destination being a different interface. A positive match leads to the
	// second isolation chain. No match returns to the parent chain. The second
	// isolation chain matches on destination interface being a bridge network's
	// bridge. A positive match identifies a packet originated from one bridge
	// network's bridge destined to another bridge network's bridge and will
	// result in the packet being dropped. No match returns to the parent chain.
	isolationChain1 = "DOCKER-ISOLATION-STAGE-1"
	isolationChain2 = "DOCKER-ISOLATION-STAGE-2"
)

type FirewallConfig struct {
	IPv4    bool
	IPv6    bool
	Hairpin bool
}

type Iptabler struct {
	FirewallConfig
}

func NewIptabler(config FirewallConfig) (*Iptabler, error) {
	ipt := &Iptabler{FirewallConfig: config}

	if ipt.IPv4 {
		removeIPChains(iptables.IPv4)

		if err := setupIPChains(iptables.IPv4, ipt.Hairpin); err != nil {
			return nil, err
		}

		// Make sure on firewall reload, first thing being re-played is chains creation
		iptables.OnReloaded(func() {
			log.G(context.TODO()).Debugf("Recreating iptables chains on firewall reload")
			if err := setupIPChains(iptables.IPv4, ipt.Hairpin); err != nil {
				log.G(context.TODO()).WithError(err).Error("Error reloading iptables chains")
			}
		})
	}

	if ipt.IPv6 {
		if err := modprobe.LoadModules(context.TODO(), func() error {
			iptable := iptables.GetIptable(iptables.IPv6)
			_, err := iptable.Raw("-t", "filter", "-n", "-L", "FORWARD")
			return err
		}, "ip6_tables"); err != nil {
			log.G(context.TODO()).WithError(err).Debug("Loading ip6_tables")
		}

		removeIPChains(iptables.IPv6)

		err := setupIPChains(iptables.IPv6, ipt.Hairpin)
		if err != nil {
			// If the chains couldn't be set up, it's probably because the kernel has no IPv6
			// support, or it doesn't have module ip6_tables loaded. It won't be possible to
			// create IPv6 networks without enabling ip6_tables in the kernel, or disabling
			// ip6tables in the daemon config. But, allow the daemon to start because IPv4
			// will work. So, log the problem, and continue.
			log.G(context.TODO()).WithError(err).Warn("ip6tables is enabled, but cannot set up ip6tables chains")
		} else {
			// Make sure on firewall reload, first thing being re-played is chains creation
			iptables.OnReloaded(func() {
				log.G(context.TODO()).Debugf("Recreating ip6tables chains on firewall reload")
				if err := setupIPChains(iptables.IPv6, ipt.Hairpin); err != nil {
					log.G(context.TODO()).WithError(err).Error("Error reloading ip6tables chains")
				}
			})
		}
	}

	return ipt, nil
}

func setupIPChains(version iptables.IPVersion, hairpin bool) (retErr error) {
	iptable := iptables.GetIptable(version)

	_, err := iptable.NewChain(dockerChain, iptables.Nat)
	if err != nil {
		return fmt.Errorf("failed to create NAT chain %s: %v", dockerChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(dockerChain, iptables.Nat); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables NAT chain %s on cleanup: %v", dockerChain, err)
			}
		}
	}()

	_, err = iptable.NewChain(dockerChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(dockerChain, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", dockerChain, err)
			}
		}
	}()

	_, err = iptable.NewChain(DockerForwardChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", DockerForwardChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(DockerForwardChain, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", DockerForwardChain, err)
			}
		}
	}()

	_, err = iptable.NewChain(dockerBridgeChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerBridgeChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(dockerBridgeChain, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", dockerBridgeChain, err)
			}
		}
	}()

	_, err = iptable.NewChain(dockerCTChain, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER chain %s: %v", dockerCTChain, err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(dockerCTChain, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", dockerCTChain, err)
			}
		}
	}()

	_, err = iptable.NewChain(isolationChain1, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(isolationChain1, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", isolationChain1, err)
			}
		}
	}()

	_, err = iptable.NewChain(isolationChain2, iptables.Filter)
	if err != nil {
		return fmt.Errorf("failed to create FILTER isolation chain: %v", err)
	}
	defer func() {
		if retErr != nil {
			if err := iptable.RemoveExistingChain(isolationChain2, iptables.Filter); err != nil {
				log.G(context.TODO()).Warnf("failed on removing iptables FILTER chain %s on cleanup: %v", isolationChain2, err)
			}
		}
	}()

	if err := addNATJumpRules(version, hairpin, true); err != nil {
		return fmt.Errorf("failed to add jump rules to %s NAT table: %w", version, err)
	}
	defer func() {
		if retErr != nil {
			if err := addNATJumpRules(version, hairpin, false); err != nil {
				log.G(context.TODO()).Warnf("failed on removing jump rules from %s NAT table: %v", version, err)
			}
		}
	}()

	// Make sure the filter-FORWARD chain has rules to accept related packets and
	// jump to the isolation and docker chains. (Re-)insert at the top of the table,
	// in reverse order.
	if err := iptable.EnsureJumpRule("FORWARD", DockerForwardChain); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(DockerForwardChain, dockerBridgeChain); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(DockerForwardChain, isolationChain1); err != nil {
		return err
	}
	if err := iptable.EnsureJumpRule(DockerForwardChain, dockerCTChain); err != nil {
		return err
	}

	if err := mirroredWSL2Workaround(version, hairpin); err != nil {
		return err
	}

	// Delete rules that may have been added to the FORWARD chain by moby 28.0.0.
	ipsetName := "docker-ext-bridges-v4"
	if version == iptables.IPv6 {
		ipsetName = "docker-ext-bridges-v6"
	}
	if err := iptable.DeleteJumpRule("FORWARD", dockerChain,
		"-m", "set", "--match-set", ipsetName, "dst"); err != nil {
		log.G(context.TODO()).WithFields(log.Fields{"error": err, "set": ipsetName}).Debug(
			"deleting legacy ipset dest match rule")
	}
	if err := iptable.DeleteJumpRule("FORWARD", isolationChain1); err != nil {
		return err
	}
	if err := iptable.DeleteJumpRule("FORWARD", "ACCEPT",
		"-m", "set", "--match-set", ipsetName, "dst",
		"-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED",
	); err != nil {
		log.G(context.TODO()).WithFields(log.Fields{"error": err, "set": ipsetName}).Debug(
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

func appendOrDelChainRule(rule iptables.Rule, ruleDescr string, append bool) error {
	operation := "disable"
	fn := rule.Delete
	if append {
		operation = "enable"
		fn = rule.Append
	}
	if err := fn(); err != nil {
		return fmt.Errorf("Unable to %s %s rule: %w", operation, ruleDescr, err)
	}
	return nil
}
