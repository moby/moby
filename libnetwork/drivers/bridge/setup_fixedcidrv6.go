package bridge

import (
	"os"

	log "github.com/Sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

func setupFixedCIDRv6(config *networkConfiguration, i *bridgeInterface) error {
	log.Debugf("Using IPv6 subnet: %v", config.FixedCIDRv6)
	if err := ipAllocator.RegisterSubnet(config.FixedCIDRv6, config.FixedCIDRv6); err != nil {
		return &FixedCIDRv6Error{Net: config.FixedCIDRv6, Err: err}
	}

	// Setting route to global IPv6 subnet
	log.Debugf("Adding route to IPv6 network %s via device %s", config.FixedCIDRv6.String(), config.BridgeName)
	err := netlink.RouteAdd(&netlink.Route{
		Scope:     netlink.SCOPE_UNIVERSE,
		LinkIndex: i.Link.Attrs().Index,
		Dst:       config.FixedCIDRv6,
	})
	if err != nil && !os.IsExist(err) {
		log.Errorf("Could not add route to IPv6 network %s via device %s", config.FixedCIDRv6.String(), config.BridgeName)
	}
	return nil
}
