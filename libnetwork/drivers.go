package libnetwork

import (
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/drivers/bridge"
	"github.com/docker/libnetwork/drivers/host"
	"github.com/docker/libnetwork/drivers/null"
)

type driverTable map[string]driverapi.Driver

func enumerateDrivers() driverTable {
	drivers := make(driverTable)

	for _, fn := range [](func() (string, driverapi.Driver)){
		bridge.New,
		host.New,
		null.New,
	} {
		name, driver := fn()
		drivers[name] = driver
	}

	return drivers
}
