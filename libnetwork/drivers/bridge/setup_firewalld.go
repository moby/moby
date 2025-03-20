//go:build linux

package bridge

import (
	"github.com/docker/docker/libnetwork/iptables"
)

func (n *bridgeNetwork) setupFirewalld(config *networkConfiguration, i *bridgeInterface) error {
	// FIXME(robmry) - these reload functions aren't deleted when the network is deleted.
	//  So, a firewalld reload leads to creation of zombie rules belonging to those networks.
	if n.driver.config.EnableIPTables && config.EnableIPv4 {
		iptables.OnReloaded(func() { n.setupIP4Tables(config, i) })
	}
	if n.driver.config.EnableIP6Tables && config.EnableIPv6 {
		iptables.OnReloaded(func() { n.setupIP6Tables(config, i) })
	}
	iptables.OnReloaded(n.reapplyPerPortIptables)
	return nil
}
