package libnetwork

import (
	"context"

	"github.com/containerd/containerd/log"
	"github.com/docker/docker/libnetwork/iptables"
)

const userChain = "DOCKER-USER"

var ctrl *Controller

func setupArrangeUserFilterRule(c *Controller) {
	ctrl = c
	iptables.OnReloaded(arrangeUserFilterRule)
}

// arrangeUserFilterRule sets up the DOCKER-USER chain for each iptables version
// (IPv4, IPv6) that's enabled in the controller's configuration.
//
// This chain allows users to configure firewall policies in a way that
// persist daemon operations/restarts. The daemon does not delete or modify
// any pre-existing rules from the DOCKER-USER filter chain.
//
// Once the DOCKER-USER chain is created, the daemon does not remove it when
// IPTableForwarding is disabled, because it contains rules configured by user
// that are beyond the daemon's control.
func arrangeUserFilterRule() {
	if ctrl == nil {
		return
	}

	for _, ipVersion := range ctrl.enabledIptablesVersions() {
		ipt := iptables.GetIptable(ipVersion)
		if _, err := ipt.NewChain(userChain, iptables.Filter, false); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to create %s %v chain", userChain, ipVersion)
			return
		}
		if err := ipt.AddReturnRule(userChain); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to add the RETURN rule for %s %v", userChain, ipVersion)
			return
		}
		if err := ipt.EnsureJumpRule("FORWARD", userChain); err != nil {
			log.G(context.TODO()).WithError(err).Warnf("Failed to ensure the jump rule for %s %v", userChain, ipVersion)
		}
	}
}
