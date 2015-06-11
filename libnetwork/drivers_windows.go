package libnetwork

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/windows"
)

func initDrivers(dc driverapi.DriverCallback) error {
	for _, fn := range [](func(driverapi.DriverCallback) error){
		windows.Init,
	} {
		if err := fn(dc); err != nil {
			return err
		}
	}
	return nil
}
