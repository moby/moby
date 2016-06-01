// +build experimental

package libnetwork

import (
	"github.com/docker/libnetwork/drivers/ipvlan"
	"github.com/docker/libnetwork/drivers/macvlan"
)

func additionalDrivers() []initializer {
	return []initializer{
		{macvlan.Init, "macvlan"},
		{ipvlan.Init, "ipvlan"},
	}
}
