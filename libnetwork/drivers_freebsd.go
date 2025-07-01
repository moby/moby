package libnetwork

import (
	"github.com/docker/docker/daemon/libnetwork/drivers/null"
	"github.com/docker/docker/libnetwork/driverapi"
)

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	return null.Register(r)
}
