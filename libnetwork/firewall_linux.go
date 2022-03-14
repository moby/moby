package libnetwork

import (
	"github.com/docker/docker/libnetwork/firewallapi"
	"github.com/docker/docker/libnetwork/firewalld"
	"github.com/docker/docker/libnetwork/iptables"
	"github.com/docker/docker/libnetwork/nftables"
	"github.com/sirupsen/logrus"
)

const userChain = "DOCKER-USER"

var (
	ctrl *controller = nil
)

func setupArrangeUserFilterRule(c *controller) {
	ctrl = c
	firewalld.OnReloaded(arrangeUserFilterRule)
}

// This chain allow users to configure firewall policies in a way that persists
// docker operations/restarts. Docker will not delete or modify any pre-existing
// rules from the DOCKER-USER filter chain.
// Note once DOCKER-USER chain is created, docker engine does not remove it when
// IPTableForwarding is disabled, because it contains rules configured by user that
// are beyond docker engine's control.
func arrangeUserFilterRule() {
	if ctrl == nil || (!ctrl.iptablesEnabled() && !ctrl.nftablesEnabled()) {
		return
	}

	var table firewallapi.FirewallTable

	if ctrl.nftablesEnabled() {
		table = nftables.GetTable(nftables.IPv4)
	} else {
		table = iptables.GetTable(iptables.IPv4)
	}

	// TODO IPv6 support
	_, err := table.NewChain(userChain, iptables.Filter, false)
	if err != nil {
		logrus.Warnf("Failed to create %s chain: %v", userChain, err)
		return
	}

	if err = table.AddReturnRule(userChain); err != nil {
		logrus.Warnf("Failed to add the RETURN rule for %s: %v", userChain, err)
		return
	}

	err = table.EnsureJumpRule("FORWARD", userChain)
	if err != nil {
		logrus.Warnf("Failed to ensure the jump rule for %s: %v", userChain, err)
	}
}
