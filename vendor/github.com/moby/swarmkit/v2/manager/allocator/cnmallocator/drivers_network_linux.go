package cnmallocator

import (
	"github.com/docker/docker/libnetwork/drivers/bridge/brmanager"
	"github.com/docker/docker/libnetwork/drivers/host"
	"github.com/docker/docker/libnetwork/drivers/ipvlan/ivmanager"
	"github.com/docker/docker/libnetwork/drivers/macvlan/mvmanager"
	"github.com/docker/docker/libnetwork/drivers/overlay/ovmanager"
	"github.com/docker/docker/libnetwork/drivers/remote"
	"github.com/moby/swarmkit/v2/manager/allocator/networkallocator"
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
