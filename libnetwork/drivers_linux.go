package libnetwork

import (
	"github.com/moby/moby/libnetwork/drivers/bridge"
	"github.com/moby/moby/libnetwork/drivers/host"
	"github.com/moby/moby/libnetwork/drivers/ipvlan"
	"github.com/moby/moby/libnetwork/drivers/macvlan"
	"github.com/moby/moby/libnetwork/drivers/null"
	"github.com/moby/moby/libnetwork/drivers/overlay"
	"github.com/moby/moby/libnetwork/drivers/remote"
)

func getInitializers(experimental bool) []initializer {
	in := []initializer{
		{bridge.Init, "bridge"},
		{host.Init, "host"},
		{ipvlan.Init, "ipvlan"},
		{macvlan.Init, "macvlan"},
		{null.Init, "null"},
		{overlay.Init, "overlay"},
		{remote.Init, "remote"},
	}
	return in
}
