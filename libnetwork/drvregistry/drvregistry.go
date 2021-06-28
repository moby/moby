package drvregistry

import (
	"fmt"

	"github.com/docker/docker/libnetwork/driverapi"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/pkg/plugingetter"
)

// DrvRegistry holds the registry of all network drivers and IPAM drivers that it knows about.
type DrvRegistry struct {
	Networks
	IPAMs
	pluginGetter plugingetter.PluginGetter
}

var (
	_ driverapi.DriverCallback = (*DrvRegistry)(nil)
	_ ipamapi.Callback         = (*DrvRegistry)(nil)
)

// InitFunc defines the driver initialization function signature.
type InitFunc func(driverapi.DriverCallback, map[string]interface{}) error

// Placeholder is a type for function arguments which need to be present for Swarmkit
// to compile, but for which the only acceptable value is nil.
type Placeholder *struct{}

// New returns a new legacy driver registry.
//
// Deprecated: use the separate [Networks] and [IPAMs] registries.
func New(dfn DriverNotifyFunc, pg plugingetter.PluginGetter) (*DrvRegistry, error) {
	return &DrvRegistry{
		Networks:     Networks{Notify: dfn},
		pluginGetter: pg,
	}, nil
}

// AddDriver adds a network driver to the registry.
//
// Deprecated: call fn(r, config) directly.
func (r *DrvRegistry) AddDriver(fn InitFunc, config map[string]interface{}) error {
	return fn(r, config)
}

// IPAMDefaultAddressSpaces returns the default address space strings for the passed IPAM driver name.
//
// Deprecated: call GetDefaultAddressSpaces() on the IPAM driver.
func (r *DrvRegistry) IPAMDefaultAddressSpaces(name string) (string, string, error) {
	d, _ := r.IPAM(name)

	if d == nil {
		return "", "", fmt.Errorf("ipam %s not found", name)
	}

	return d.GetDefaultAddressSpaces()
}

// GetPluginGetter returns the plugingetter
func (r *DrvRegistry) GetPluginGetter() plugingetter.PluginGetter {
	return r.pluginGetter
}

// Driver returns the network driver instance registered under name, and its capability.
func (r *DrvRegistry) Driver(name string) (driverapi.Driver, *driverapi.Capability) {
	d, c := r.Networks.Driver(name)

	if c == (driverapi.Capability{}) {
		return d, nil
	}
	return d, &c
}
