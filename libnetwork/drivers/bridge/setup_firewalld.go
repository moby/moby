//go:build linux

package bridge

import (
	"github.com/docker/docker/libnetwork/iptables"
)

func (n *bridgeNetwork) setupFirewalld(config *networkConfiguration, i *bridgeInterface) error {
	// FIXME(robmry) - these reload functions aren't deleted when the network is deleted.
	//  So, a firewalld reload leads to creation of zombie rules belonging to those networks.
	iptables.OnReloaded(func() { n.iptablesNetwork.reapplyNetworkLevelRules() })
	iptables.OnReloaded(n.reapplyPerPortIptables)
	return nil
}
