package drvregistry

import (
	"errors"
	"strings"
	"sync"

	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
)

// DriverWalkFunc defines the network driver table walker function signature.
type DriverWalkFunc func(name string, driver driverapi.Driver, capability driverapi.Capability) bool

type driverData struct {
	driver     driverapi.Driver
	capability driverapi.Capability
}

// Networks is a registry of network drivers. The zero value is an empty network
// driver registry, ready to use.
type Networks struct {
	// Notify is called whenever a network driver is registered.
	Notify driverapi.Registerer

	mu       sync.Mutex
	drivers  map[string]driverData
	nwAllocs map[string]driverapi.NetworkAllocator
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
		if err := nr.Notify.RegisterDriver(ntype, driver, capability); err != nil {
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

// NetworkAllocator returns the NetworkAllocator registered under name, and its capability.
func (nr *Networks) NetworkAllocator(name string) driverapi.NetworkAllocator {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	d := nr.nwAllocs[name]
	return d
}

func (nr *Networks) RegisterNetworkAllocator(ntype string, nwAlloc driverapi.NetworkAllocator) error {
	if strings.TrimSpace(ntype) == "" {
		return errors.New("network type string cannot be empty")
	}

	nr.mu.Lock()
	dd, ok := nr.nwAllocs[ntype]
	nr.mu.Unlock()

	if ok && dd.IsBuiltIn() {
		return driverapi.ErrActiveRegistration(ntype)
	}

	if nr.Notify != nil {
		if err := nr.Notify.RegisterNetworkAllocator(ntype, nwAlloc); err != nil {
			return err
		}
	}

	nr.mu.Lock()
	defer nr.mu.Unlock()

	if nr.nwAllocs == nil {
		nr.nwAllocs = make(map[string]driverapi.NetworkAllocator)
	}
	nr.nwAllocs[ntype] = nwAlloc

	return nil
}

func (nr *Networks) HasDriverOrNwAllocator(ntype string) bool {
	nr.mu.Lock()
	defer nr.mu.Unlock()

	_, hasDriver := nr.drivers[ntype]
	_, hasNwAlloc := nr.nwAllocs[ntype]
	return hasDriver || hasNwAlloc
}
