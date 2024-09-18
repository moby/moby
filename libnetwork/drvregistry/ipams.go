package drvregistry

import (
	"errors"
	"strings"
	"sync"

	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/types"
)

type ipamDriver struct {
	driver     ipamapi.Ipam
	capability *ipamapi.Capability
}

// IPAMs is a registry of IPAM drivers. The zero value is an empty IPAM driver
// registry, ready to use.
type IPAMs struct {
	mu      sync.Mutex
	drivers map[string]ipamDriver
}

var _ ipamapi.Registerer = (*IPAMs)(nil)

// IPAM returns the actual IPAM driver instance and its capability which registered with the passed name.
func (ir *IPAMs) IPAM(name string) (ipamapi.Ipam, *ipamapi.Capability) {
	ir.mu.Lock()
	defer ir.mu.Unlock()

	d := ir.drivers[name]
	return d.driver, d.capability
}

// RegisterIpamDriverWithCapabilities registers the IPAM driver discovered with specified capabilities.
func (ir *IPAMs) RegisterIpamDriverWithCapabilities(name string, driver ipamapi.Ipam, caps *ipamapi.Capability) error {
	if strings.TrimSpace(name) == "" {
		return errors.New("ipam driver name string cannot be empty")
	}

	ir.mu.Lock()
	defer ir.mu.Unlock()

	dd, ok := ir.drivers[name]
	if ok && dd.driver.IsBuiltIn() {
		return types.ForbiddenErrorf("ipam driver %q already registered", name)
	}

	if ir.drivers == nil {
		ir.drivers = make(map[string]ipamDriver)
	}
	ir.drivers[name] = ipamDriver{driver: driver, capability: caps}

	return nil
}

// RegisterIpamDriver registers the IPAM driver discovered with default capabilities.
func (ir *IPAMs) RegisterIpamDriver(name string, driver ipamapi.Ipam) error {
	return ir.RegisterIpamDriverWithCapabilities(name, driver, &ipamapi.Capability{})
}

// IPAMWalkFunc defines the IPAM driver table walker function signature.
type IPAMWalkFunc func(name string, driver ipamapi.Ipam, cap *ipamapi.Capability) bool

// WalkIPAMs walks the IPAM drivers registered in the registry and invokes the passed walk function and each one of them.
func (ir *IPAMs) WalkIPAMs(ifn IPAMWalkFunc) {
	type ipamVal struct {
		name string
		data ipamDriver
	}

	ir.mu.Lock()
	ivl := make([]ipamVal, 0, len(ir.drivers))
	for k, v := range ir.drivers {
		ivl = append(ivl, ipamVal{name: k, data: v})
	}
	ir.mu.Unlock()

	for _, iv := range ivl {
		if ifn(iv.name, iv.data.driver, iv.data.capability) {
			break
		}
	}
}
