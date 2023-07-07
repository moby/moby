package cnmallocator

import (
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/bridge/brmanager"
	"github.com/docker/docker/libnetwork/drivers/host"
	"github.com/docker/docker/libnetwork/drivers/ipvlan/ivmanager"
	"github.com/docker/docker/libnetwork/drivers/macvlan/mvmanager"
	"github.com/docker/docker/libnetwork/drivers/overlay/ovmanager"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

var initializers = map[string]func(driverapi.Registerer) error{
	"overlay": ovmanager.Register,
	"macvlan": mvmanager.Register,
	"bridge":  brmanager.Register,
	"ipvlan":  ivmanager.Register,
	"host":    host.Register,
}

// PredefinedNetworks returns the list of predefined network structures
func PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return []networkallocator.PredefinedNetworkData{
		{Name: "bridge", Driver: "bridge"},
		{Name: "host", Driver: "host"},
	}
}
