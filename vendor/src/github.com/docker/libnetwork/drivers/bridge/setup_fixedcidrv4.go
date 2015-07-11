package bridge

import (
	log "github.com/Sirupsen/logrus"
)

func setupFixedCIDRv4(config *networkConfiguration, i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return err
	}

	log.Debugf("Using IPv4 subnet: %v", config.FixedCIDR)
	if err := ipAllocator.RegisterSubnet(addrv4.IPNet, config.FixedCIDR); err != nil {
		return &FixedCIDRv4Error{Subnet: config.FixedCIDR, Net: addrv4.IPNet, Err: err}
	}

	return nil
}
