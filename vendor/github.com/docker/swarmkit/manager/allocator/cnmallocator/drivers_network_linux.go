package cnmallocator

import (
	"github.com/moby/moby/libnetwork/drivers/bridge/brmanager"
	"github.com/moby/moby/libnetwork/drivers/host"
	"github.com/moby/moby/libnetwork/drivers/ipvlan/ivmanager"
	"github.com/moby/moby/libnetwork/drivers/macvlan/mvmanager"
	"github.com/moby/moby/libnetwork/drivers/overlay/ovmanager"
	"github.com/moby/moby/libnetwork/drivers/remote"
	"github.com/docker/swarmkit/manager/allocator/networkallocator"
)

var initializers = []initializer{
	{remote.Init, "remote"},
	{ovmanager.Init, "overlay"},
	{mvmanager.Init, "macvlan"},
	{brmanager.Init, "bridge"},
	{ivmanager.Init, "ipvlan"},
	{host.Init, "host"},
}

// PredefinedNetworks returns the list of predefined network structures
func PredefinedNetworks() []networkallocator.PredefinedNetworkData {
	return []networkallocator.PredefinedNetworkData{
		{Name: "bridge", Driver: "bridge"},
		{Name: "host", Driver: "host"},
	}
}
