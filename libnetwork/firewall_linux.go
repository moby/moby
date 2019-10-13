package libnetwork

import (
	"github.com/docker/libnetwork/iptables"
	"github.com/sirupsen/logrus"
)

const userChain = "DOCKER-USER"

var (
	ctrl *controller = nil
)

func setupArrangeUserFilterRule(c *controller) {
	ctrl = c
	iptables.OnReloaded(arrangeUserFilterRule)
}

// This chain allow users to configure firewall policies in a way that persists
// docker operations/restarts. Docker will not delete or modify any pre-existing
// rules from the DOCKER-USER filter chain.
// Note once DOCKER-USER chain is created, docker engine does not remove it when
// IPTableForwarding is disabled, because it contains rules configured by user that
// are beyond docker engine's control.
func arrangeUserFilterRule() {
	if ctrl == nil || !ctrl.iptablesEnabled() {
		return
	}
	_, err := iptables.NewChain(userChain, iptables.Filter, false)
	if err != nil {
		logrus.Warnf("Failed to create %s chain: %v", userChain, err)
		return
	}

	if err = iptables.AddReturnRule(userChain); err != nil {
		logrus.Warnf("Failed to add the RETURN rule for %s: %v", userChain, err)
		return
	}

	err = iptables.EnsureJumpRule("FORWARD", userChain)
	if err != nil {
		logrus.Warnf("Failed to ensure the jump rule for %s: %v", userChain, err)
	}
}
