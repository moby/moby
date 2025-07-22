package libnetwork

import (
	"github.com/moby/moby/daemon/libnetwork/driverapi"
	"github.com/moby/moby/daemon/libnetwork/drivers/null"
)

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	return null.Register(r)
}
