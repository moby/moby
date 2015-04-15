package bridge

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
)

func setupFixedCIDRv6(config *Configuration, i *bridgeInterface) error {
	log.Debugf("Using IPv6 subnet: %v", config.FixedCIDRv6)
	if err := ipAllocator.RegisterSubnet(config.FixedCIDRv6, config.FixedCIDRv6); err != nil {
		return fmt.Errorf("Setup FixedCIDRv6 failed for subnet %s in %s: %v", config.FixedCIDRv6, config.FixedCIDRv6, err)
	}

	return nil
}
