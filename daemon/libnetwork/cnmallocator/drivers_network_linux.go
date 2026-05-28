package cnmallocator

import (
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/bridge"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/host"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/ipvlan"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/macvlan"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/overlay/ovmanager"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
)

// globalDrivers is a map of network drivers that support cluster-wide
// definition and require cluster-wide resources allocation (i.e. DataScope == scope.Global).
var globalDrivers = map[string]func(driverapi.Registerer) error{
	"overlay": ovmanager.Register,
}

// localDrivers is a list of builtin network drivers that support cluster-wide
// definition (i.e. --scope=swarm on the CLI), but don't need global
// resources allocations (i.e., DataScope == scope.Local).
var localDrivers = []string{
	bridge.NetworkType,
	host.NetworkType,
	ipvlan.NetworkType,
	macvlan.NetworkType,
}

// PredefinedNetworks returns the list of predefined network structures
func (*Provider) PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return []networkallocator.PredefinedNetworkData{
		{Name: "bridge", Driver: "bridge"},
		{Name: "host", Driver: "host"},
	}
}
