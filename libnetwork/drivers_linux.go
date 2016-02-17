package libnetwork

import (
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/drivers/host"
	"github.com/docker/libnetwork/drivers/ipvlan"
	"github.com/docker/libnetwork/drivers/macvlan"
	"github.com/docker/libnetwork/drivers/null"
	"github.com/docker/libnetwork/drivers/overlay"
	"github.com/docker/libnetwork/drivers/remote"
)

func getInitializers() []initializer {
	return []initializer{
		{bridge.Init, "bridge"},
		{host.Init, "host"},
		{null.Init, "null"},
		{remote.Init, "remote"},
		{overlay.Init, "overlay"},
		{macvlan.Init, "macvlan"},
		{ipvlan.Init, "ipvlan"},
	}
}
