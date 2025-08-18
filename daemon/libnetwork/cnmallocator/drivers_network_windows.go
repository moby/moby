package cnmallocator

import (
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay/ovmanager"
)

var initializers = map[string]func(driverapi.Registerer) error{
	"overlay":  ovmanager.Register,
	"internal": stubManager("internal"),
	"l2bridge": stubManager("l2bridge"),
	"nat":      stubManager("nat"),
}

// PredefinedNetworks returns the list of predefined network structures
func (*Provider) PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return []networkallocator.PredefinedNetworkData{
		{Name: "nat", Driver: "nat"},
	}
}

func stubManager(ntype string) func(driverapi.Registerer) error {
	return func(r driverapi.Registerer) error {
		return RegisterManager(r, ntype)
	}
}
