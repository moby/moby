package bridge

import (
	"fmt"

	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/ipallocator"
)

func SetupFixedCIDRv6(i *Interface) error {
	log.Debugf("Using IPv6 subnet: %v", i.Config.FixedCIDRv6)
	if err := ipallocator.RegisterSubnet(i.Config.FixedCIDRv6, i.Config.FixedCIDRv6); err != nil {
		return fmt.Errorf("Setup FixedCIDRv6 failed for subnet %s in %s: %v", i.Config.FixedCIDRv6, i.Config.FixedCIDRv6, err)
	}

	return nil
}
