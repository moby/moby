package networkallocator

import (
	"github.com/docker/libnetwork/drivers/overlay/ovmanager"
	"github.com/docker/libnetwork/drivers/remote"
	"github.com/docker/libnetwork/drvregistry"
)

const initializers = []initializer{
	{remote.Init, "remote"},
	{ovmanager.Init, "overlay"},
}

// PredefinedNetworks returns the list of predefined network structures
func PredefinedNetworks() []PredefinedNetworkData {
	return nil
}
