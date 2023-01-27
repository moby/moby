package libnetwork

import (
	"github.com/docker/docker/libnetwork/drivers/bridge"
	"github.com/docker/docker/libnetwork/drivers/host"
	"github.com/docker/docker/libnetwork/drivers/ipvlan"
	"github.com/docker/docker/libnetwork/drivers/macvlan"
	"github.com/docker/docker/libnetwork/drivers/null"
	"github.com/docker/docker/libnetwork/drivers/overlay"
)

func getInitializers() []initializer {
	in := []initializer{
		{bridge.Register, "bridge"},
		{host.Register, "host"},
		{ipvlan.Register, "ipvlan"},
		{macvlan.Register, "macvlan"},
		{null.Register, "null"},
		{overlay.Register, "overlay"},
	}
	return in
}
