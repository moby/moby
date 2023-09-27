package libnetwork

import (
	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/drivers/host"
	"github.com/docker/docker/libnetwork/drivers/null"
)

func registerNetworkDrivers(r driverapi.Registerer, driverConfig func(string) map[string]interface{}) error {
	err := null.Register(r)
	if err != nil {
		return err
	}

	err = host.Register(r)
	if err != nil {
		return err
	}

	return nil
}
