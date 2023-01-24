package drvregistry

import (
	"errors"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/driverapi"
)

// DriverWalkFunc defines the network driver table walker function signature.
type DriverWalkFunc func(name string, driver driverapi.Driver, capability driverapi.Capability) bool

// DriverNotifyFunc defines the notify function signature when a new network driver gets registered.
type DriverNotifyFunc func(name string, driver driverapi.Driver, capability driverapi.Capability) error

type driverData struct {
	driver     driverapi.Driver
	capability driverapi.Capability
}

// Networks is a registry of network drivers. The zero value is an empty network
// driver registry, ready to use.
type Networks struct {
	// Notify is called whenever a network driver is registered.
	Notify DriverNotifyFunc

	mu      sync.Mutex
	drivers map[string]driverData
}

var _ driverapi.Registerer = (*Networks)(nil)

// WalkDrivers walks the network drivers registered in the registry and invokes the passed walk function and each one of them.
func (nr *Networks) WalkDrivers(dfn DriverWalkFunc) {
	type driverVal struct {
		name string
		data driverData
	}

	nr.mu.Lock()
	dvl := make([]driverVal, 0, len(nr.drivers))
	for k, v := range nr.drivers {
		dvl = append(dvl, driverVal{name: k, data: v})
	}
	nr.mu.Unlock()

	for _, dv := range dvl {
		if dfn(dv.name, dv.data.driver, dv.data.capability) {
			break
		}
	}
}

// Driver returns the network driver instance registered under name, and its capability.
func (nr *Networks) Driver(name string) (driverapi.Driver, driverapi.Capability) {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	d := nr.drivers[name]
	return d.driver, d.capability
}

// RegisterDriver registers the network driver with nr.
func (nr *Networks) RegisterDriver(ntype string, driver driverapi.Driver, capability driverapi.Capability) error {
	if strings.TrimSpace(ntype) == "" {
		return errors.New("network type string cannot be empty")
	}

	nr.mu.Lock()
	dd, ok := nr.drivers[ntype]
	nr.mu.Unlock()

	if ok && dd.driver.IsBuiltIn() {
		return driverapi.ErrActiveRegistration(ntype)
	}

	if nr.Notify != nil {
		if err := nr.Notify(ntype, driver, capability); err != nil {
			return err
		}
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()

	if nr.drivers == nil {
		nr.drivers = make(map[string]driverData)
	}
	nr.drivers[ntype] = driverData{driver: driver, capability: capability}

	return nil
}
