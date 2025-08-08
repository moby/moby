package libnetwork

import (
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drivers/null"
)

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]any) error {
	return null.Register(r)
}
