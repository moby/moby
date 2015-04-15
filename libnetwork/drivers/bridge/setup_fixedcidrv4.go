package bridge

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
)

func setupFixedCIDRv4(config *Configuration, i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return err
	}

	log.Debugf("Using IPv4 subnet: %v", config.FixedCIDR)
	if err := ipAllocator.RegisterSubnet(addrv4.IPNet, config.FixedCIDR); err != nil {
		return fmt.Errorf("Setup FixedCIDRv4 failed for subnet %s in %s: %v", config.FixedCIDR, addrv4.IPNet, err)
	}

	return nil
}
