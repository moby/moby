package libnetwork

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/drivers/host"
	"github.com/docker/libnetwork/drivers/null"
	"github.com/docker/libnetwork/drivers/remote"
)

type driverTable map[string]driverapi.Driver

func enumerateDrivers(dc driverapi.DriverCallback) driverTable {
	drivers := make(driverTable)

	for _, fn := range [](func(driverapi.DriverCallback) (string, driverapi.Driver)){
		bridge.New,
		host.New,
		null.New,
		remote.New,
	} {
		name, driver := fn(dc)
		drivers[name] = driver
	}

	return drivers
}
