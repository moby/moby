package libnetwork

import (
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/drivers/host"
	"github.com/docker/libnetwork/drivers/macvlan"
	"github.com/docker/libnetwork/drivers/null"
	"github.com/docker/libnetwork/drivers/overlay"
	"github.com/docker/libnetwork/drivers/remote"
        "github.com/docker/libnetwork/drivers/network_bandwidth"
)

func getInitializers() []initializer {
	in := []initializer{
		{bridge.Init, "bridge"},
		{host.Init, "host"},
		{macvlan.Init, "macvlan"},
		{null.Init, "null"},
		{remote.Init, "remote"},
		{overlay.Init, "overlay"},
                {bandwidth_drv.Init, "bandwidth_drv"},
	}

	in = append(in, additionalDrivers()...)
	return in
}
