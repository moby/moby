package bridge

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/daemon/networkdriver/ipallocator"
)

func SetupFixedCIDRv6(i *Interface) error {
	log.Debugf("Using IPv6 subnet: %v", i.Config.FixedCIDRv6)
	return ipallocator.RegisterSubnet(i.Config.FixedCIDRv6, i.Config.FixedCIDRv6)
}
