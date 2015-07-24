package bridge

import "github.com/docker/libnetwork/iptables"

func (n *bridgeNetwork) setupFirewalld(config *networkConfiguration, i *bridgeInterface) error {
	// Sanity check.
	if config.EnableIPTables == false {
		return IPTableCfgError(config.BridgeName)
	}

	iptables.OnReloaded(func() { n.setupIPTables(config, i) })
	iptables.OnReloaded(n.portMapper.ReMapAll)

	return nil
}
