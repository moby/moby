package bridge

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/ipallocator"
)

func SetupFixedCIDRv4(i *Interface) error {
	addrv4, _, err := i.Addresses()
	if err != nil {
		return err
	}

	log.Debugf("Using IPv4 subnet: %v", i.Config.FixedCIDR)
	return ipallocator.RegisterSubnet(addrv4.IPNet, i.Config.FixedCIDR)
}
