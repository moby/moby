package drvregistry

import (
	"fmt"
	"strings"
	"sync"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/types"

	builtinIpam "github.com/docker/libnetwork/ipams/builtin"
	nullIpam "github.com/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/libnetwork/ipams/remote"
)

type driverData struct {
	driver     driverapi.Driver
	capability driverapi.Capability
}

type ipamData struct {
	driver     ipamapi.Ipam
	capability *ipamapi.Capability
	// default address spaces are provided by ipam driver at registration time
	defaultLocalAddressSpace, defaultGlobalAddressSpace string
}

type driverTable map[string]*driverData
type ipamTable map[string]*ipamData

// DrvRegistry holds the registry of all network drivers and IPAM drivers that it knows about.
type DrvRegistry struct {
	sync.Mutex
	drivers     driverTable
	ipamDrivers ipamTable
	dfn         DriverNotifyFunc
	ifn         IPAMNotifyFunc
}

// Functors definition

// InitFunc defines the driver initialization function signature.
type InitFunc func(driverapi.DriverCallback, map[string]interface{}) error

// IPAMWalkFunc defines the IPAM driver table walker function signature.
type IPAMWalkFunc func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) bool

// DriverWalkFunc defines the network driver table walker function signature.
type DriverWalkFunc func(name string, driver driverapi.Driver, capability driverapi.Capability) bool

// IPAMNotifyFunc defines the notify function signature when a new IPAM driver gets registered.
type IPAMNotifyFunc func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) error

// DriverNotifyFunc defines the notify function signature when a new network driver gets registered.
type DriverNotifyFunc func(name string, driver driverapi.Driver, capability driverapi.Capability) error

// New retruns a new driver registry handle.
func New(lDs, gDs interface{}, dfn DriverNotifyFunc, ifn IPAMNotifyFunc) (*DrvRegistry, error) {
	r := &DrvRegistry{
		drivers:     make(driverTable),
		ipamDrivers: make(ipamTable),
		dfn:         dfn,
		ifn:         ifn,
	}

	if err := r.initIPAMs(lDs, gDs); err != nil {
		return nil, err
	}

	return r, nil
}

// AddDriver adds a network driver to the registry.
func (r *DrvRegistry) AddDriver(ntype string, fn InitFunc, config map[string]interface{}) error {
	return fn(r, config)
}

// WalkIPAMs walks the IPAM drivers registered in the registry and invokes the passed walk function and each one of them.
func (r *DrvRegistry) WalkIPAMs(ifn IPAMWalkFunc) {
	type ipamVal struct {
		name string
		data *ipamData
	}

	r.Lock()
	ivl := make([]ipamVal, 0, len(r.ipamDrivers))
	for k, v := range r.ipamDrivers {
		ivl = append(ivl, ipamVal{name: k, data: v})
	}
	r.Unlock()

	for _, iv := range ivl {
		if ifn(iv.name, iv.data.driver, iv.data.capability) {
			break
		}
	}
}

// WalkDrivers walks the network drivers registered in the registry and invokes the passed walk function and each one of them.
func (r *DrvRegistry) WalkDrivers(dfn DriverWalkFunc) {
	type driverVal struct {
		name string
		data *driverData
	}

	r.Lock()
	dvl := make([]driverVal, 0, len(r.drivers))
	for k, v := range r.drivers {
		dvl = append(dvl, driverVal{name: k, data: v})
	}
	r.Unlock()

	for _, dv := range dvl {
		if dfn(dv.name, dv.data.driver, dv.data.capability) {
			break
		}
	}
}

// Driver returns the actual network driver instance and its capability  which registered with the passed name.
func (r *DrvRegistry) Driver(name string) (driverapi.Driver, *driverapi.Capability) {
	r.Lock()
	defer r.Unlock()

	d, ok := r.drivers[name]
	if !ok {
		return nil, nil
	}

	return d.driver, &d.capability
}

// IPAM returns the actual IPAM driver instance and its capability which registered with the passed name.
func (r *DrvRegistry) IPAM(name string) (ipamapi.Ipam, *ipamapi.Capability) {
	r.Lock()
	defer r.Unlock()

	i, ok := r.ipamDrivers[name]
	if !ok {
		return nil, nil
	}

	return i.driver, i.capability
}

// IPAMDefaultAddressSpaces returns the default address space strings for the passed IPAM driver name.
func (r *DrvRegistry) IPAMDefaultAddressSpaces(name string) (string, string, error) {
	r.Lock()
	defer r.Unlock()

	i, ok := r.ipamDrivers[name]
	if !ok {
		return "", "", fmt.Errorf("ipam %s not found", name)
	}

	return i.defaultLocalAddressSpace, i.defaultGlobalAddressSpace, nil
}

func (r *DrvRegistry) initIPAMs(lDs, gDs interface{}) error {
	for _, fn := range [](func(ipamapi.Callback, interface{}, interface{}) error){
		builtinIpam.Init,
		remoteIpam.Init,
		nullIpam.Init,
	} {
		if err := fn(r, lDs, gDs); err != nil {
			return err
		}
	}

	return nil
}

// RegisterDriver registers the network driver when it gets discovered.
func (r *DrvRegistry) RegisterDriver(ntype string, driver driverapi.Driver, capability driverapi.Capability) error {
	if strings.TrimSpace(ntype) == "" {
		return fmt.Errorf("network type string cannot be empty")
	}

	r.Lock()
	_, ok := r.drivers[ntype]
	r.Unlock()

	if ok {
		return driverapi.ErrActiveRegistration(ntype)
	}

	if r.dfn != nil {
		if err := r.dfn(ntype, driver, capability); err != nil {
			return err
		}
	}

	dData := &driverData{driver, capability}

	r.Lock()
	r.drivers[ntype] = dData
	r.Unlock()

	return nil
}

func (r *DrvRegistry) registerIpamDriver(name string, driver ipamapi.Ipam, caps *ipamapi.Capability) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("ipam driver name string cannot be empty")
	}

	r.Lock()
	_, ok := r.ipamDrivers[name]
	r.Unlock()
	if ok {
		return types.ForbiddenErrorf("ipam driver %q already registered", name)
	}

	locAS, glbAS, err := driver.GetDefaultAddressSpaces()
	if err != nil {
		return types.InternalErrorf("ipam driver %q failed to return default address spaces: %v", name, err)
	}

	if r.ifn != nil {
		if err := r.ifn(name, driver, caps); err != nil {
			return err
		}
	}

	r.Lock()
	r.ipamDrivers[name] = &ipamData{driver: driver, defaultLocalAddressSpace: locAS, defaultGlobalAddressSpace: glbAS, capability: caps}
	r.Unlock()

	return nil
}

// RegisterIpamDriver registers the IPAM driver discovered with default capabilities.
func (r *DrvRegistry) RegisterIpamDriver(name string, driver ipamapi.Ipam) error {
	return r.registerIpamDriver(name, driver, &ipamapi.Capability{})
}

// RegisterIpamDriverWithCapabilities registers the IPAM driver discovered with specified capabilities.
func (r *DrvRegistry) RegisterIpamDriverWithCapabilities(name string, driver ipamapi.Ipam, caps *ipamapi.Capability) error {
	return r.registerIpamDriver(name, driver, caps)
}
