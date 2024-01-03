package cnmallocator

import (
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/overlay/ovmanager"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

var initializers = map[string]func(driverapi.Registerer) error{
	"overlay":  ovmanager.Register,
	"internal": stubManager("internal"),
	"l2bridge": stubManager("l2bridge"),
	"nat":      stubManager("nat"),
}

// PredefinedNetworks returns the list of predefined network structures
func PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return []networkallocator.PredefinedNetworkData{
		{Name: "nat", Driver: "nat"},
	}
}

func stubManager(ntype string) func(driverapi.Registerer) error {
	return func(r driverapi.Registerer) error {
		return RegisterManager(r, ntype)
	}
}
