package libnetwork

import (
	"github.com/docker/docker/libnetwork/drivers/null"
)

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	return null.Register(r, driverConfig(null.NetworkType))
}
