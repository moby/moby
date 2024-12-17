package libnetwork

import (
	"context"
	"errors"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

// Sets up the DOCKER-USER chain for each iptables version (IPv4, IPv6) that's
// enabled in the controller's configuration.
func (c *Controller) setupUserChains() {
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
	if err := ipt.AddReturnRule(userChain); err != nil {
		return fmt.Errorf("failed to add the RETURN rule for %s %v: %w", userChain, ipVersion, err)
	}
	if err := ipt.EnsureJumpRule("FORWARD", userChain); err != nil {
		return fmt.Errorf("failed to ensure the jump rule for %s %v: %w", userChain, ipVersion, err)
	}
	return nil
}
