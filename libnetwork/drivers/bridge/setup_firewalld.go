//go:build linux

package bridge

import (
	"context"

	"github.com/containerd/log"
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/iptables"
)

var _ driverapi.FirewallReplayer = (*driver)(nil)

func (d *driver) ReplayFirewallConfig() {
	// Make sure on firewall reload, first thing being re-played is chains creation
	if d.config.EnableIPTables {
		log.G(context.TODO()).Debugf("Recreating iptables chains on firewall reload")
		if _, _, _, _, err := setupIPChains(d.config, iptables.IPv4); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error reloading iptables chains")
		}
	}
	if d.config.EnableIP6Tables {
		log.G(context.TODO()).Debugf("Recreating ip6tables chains on firewall reload")
		if _, _, _, _, err := setupIPChains(d.config, iptables.IPv6); err != nil {
			log.G(context.TODO()).WithError(err).Error("Error reloading ip6tables chains")
		}
	}

	replaySetupIPForwarding(d.config.EnableIPTables, d.config.EnableIP6Tables)

	for _, n := range d.getNetworks() {
		n.replayFirewallConfig()
	}
}

func (n *bridgeNetwork) replayFirewallConfig() {
	if n.driver.config.EnableIPTables {
		n.setupIP4Tables(n.config, n.bridge)
		n.portMapper.ReMapAll()
	}

	if n.config.EnableIPv6 && n.driver.config.EnableIP6Tables {
		n.setupIP6Tables(n.config, n.bridge)
		n.portMapperV6.ReMapAll()
	}

	for _, ep := range n.endpoints {
		if ep.isLinked {
			_ = n.link(ep, true)
		}
	}
}
