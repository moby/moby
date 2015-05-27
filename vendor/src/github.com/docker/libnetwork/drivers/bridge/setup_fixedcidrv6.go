package bridge

import (
	log "github.com/Sirupsen/logrus"
)

func setupFixedCIDRv6(config *NetworkConfiguration, i *bridgeInterface) error {
	log.Debugf("Using IPv6 subnet: %v", config.FixedCIDRv6)
	if err := ipAllocator.RegisterSubnet(config.FixedCIDRv6, config.FixedCIDRv6); err != nil {
		return &FixedCIDRv6Error{Net: config.FixedCIDRv6, Err: err}
	}

	return nil
}
