package cnmallocator

import (
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay/ovmanager"
)

var initializers = map[string]func(driverapi.Registerer) error{
	"overlay": ovmanager.Register,
}

// PredefinedNetworks returns the list of predefined network structures
func (*Provider) PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return nil
}
