package libnetwork

import (
	"context"
	"fmt"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

func (c *Controller) setupArrangeUserFilterRule() {
	iptables.OnReloaded(func() { arrangeUserFilterRule(c.enabledIptablesVersions()...) })
}

// arrangeUserFilterRule sets up the DOCKER-USER chain for each iptables version
// (IPv4, IPv6) specified in the arguments.
func arrangeUserFilterRule(ipVersions ...iptables.IPVersion) {
	for _, ipVersion := range ipVersions {
		if err := setupUserChain(ipVersion); err != nil {
			log.G(context.TODO()).WithError(err).Warn("arrangeUserFilterRule")
		}
	}
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
	if _, err := ipt.NewChain(userChain, iptables.Filter, false); err != nil {
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
