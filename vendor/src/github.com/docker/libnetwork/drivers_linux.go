package libnetwork

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/drivers/host"
	"github.com/docker/libnetwork/drivers/null"
	o "github.com/docker/libnetwork/drivers/overlay"
	"github.com/docker/libnetwork/drivers/remote"
)

func initDrivers(dc driverapi.DriverCallback) error {
	for _, fn := range [](func(driverapi.DriverCallback) error){
		bridge.Init,
		host.Init,
		null.Init,
		remote.Init,
		o.Init,
	} {
		if err := fn(dc); err != nil {
			return err
		}
	}
	return nil
}
