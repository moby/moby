package bridge

import log "github.com/Sirupsen/logrus"

func setupFixedCIDRv4(config *NetworkConfiguration, i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return err
	}

	log.Debugf("Using IPv4 subnet: %v", config.FixedCIDR)
	if err := ipAllocator.RegisterSubnet(addrv4.IPNet, config.FixedCIDR); err != nil {
		return &FixedCIDRv4Error{subnet: config.FixedCIDR, net: addrv4.IPNet, err: err}
	}

	return nil
}
