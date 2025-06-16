package libnetwork

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/daemon/libnetwork/internal/nftables"
	"github.com/docker/docker/daemon/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

func (c *Controller) selectFirewallBackend() error {
	// If explicitly configured to use iptables, don't consider nftables.
	if c.cfg.FirewallBackend == "iptables" {
		return nil
	}
	// If configured to use nftables, but it can't be initialised, return an error.
	if c.cfg.FirewallBackend == "nftables" {
		if err := nftables.Enable(); err != nil {
			return fmt.Errorf("firewall-backend is set to nftables: %v", err)
		}
		return nil
	}
	return nil
}

// Sets up the DOCKER-USER chain for each iptables version (IPv4, IPv6) that's
// enabled in the controller's configuration.
func (c *Controller) setupUserChains() {
	// There's no equivalent to DOCKER-USER in the nftables implementation.
	if nftables.Enabled() {
		return
	}

	setup := func() error {
		var errs []error
		for _, ipVersion := range c.enabledIptablesVersions() {
			errs = append(errs, setupUserChain(ipVersion))
		}
		return errors.Join(errs...)
	}
	if err := setup(); err != nil {
		log.G(context.Background()).WithError(err).Warn("configuring " + userChain)
	}
	iptables.OnReloaded(func() {
		if err := setup(); err != nil {
			log.G(context.Background()).WithError(err).Warn("configuring " + userChain + " on firewall reload")
		}
	})
}

// setupUserChain sets up the DOCKER-USER chain for the given [iptables.IPVersion].
//
// This chain allows users to configure firewall policies in a way that
// persist daemon operations/restarts. The daemon does not delete or modify
// any pre-existing rules from the DOCKER-USER filter chain.
//
// Once the DOCKER-USER chain is created, the daemon does not remove it when
// IPTableForwarding is disabled, because it contains rules configured by user
// that are beyond the daemon's control.
func setupUserChain(ipVersion iptables.IPVersion) error {
	ipt := iptables.GetIptable(ipVersion)
	if _, err := ipt.NewChain(userChain, iptables.Filter); err != nil {
		return fmt.Errorf("failed to create %s %v chain: %v", userChain, ipVersion, err)
	}
	if err := ipt.EnsureJumpRule("FORWARD", userChain); err != nil {
		return fmt.Errorf("failed to ensure the jump rule for %s %v: %w", userChain, ipVersion, err)
	}
	return nil
}
