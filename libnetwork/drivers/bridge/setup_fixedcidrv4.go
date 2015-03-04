package bridge

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/ipallocator"
)

func setupFixedCIDRv4(i *bridgeInterface) error {
	addrv4, _, err := i.addresses()
	if err != nil {
		return err
	}

	log.Debugf("Using IPv4 subnet: %v", i.Config.FixedCIDR)
	if err := ipallocator.RegisterSubnet(addrv4.IPNet, i.Config.FixedCIDR); err != nil {
		return fmt.Errorf("Setup FixedCIDRv4 failed for subnet %s in %s: %v", i.Config.FixedCIDR, addrv4.IPNet, err)
	}

	return nil
}
