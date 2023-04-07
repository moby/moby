package builtin

import (
	"github.com/docker/docker/libnetwork/ipam"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
)

var (
	// defaultAddressPool Stores user configured subnet list
	defaultAddressPool []*ipamutils.NetworkToSplit
)

// registerBuiltin registers the built-in ipam driver with libnetwork.
func registerBuiltin(ic ipamapi.Registerer) error {
	var localAddressPool ipamutils.Subnetter
	if len(defaultAddressPool) > 0 {
		var err error
		localAddressPool, err = ipamutils.NewSubnetter(defaultAddressPool)
		if err != nil {
			return err
		}
	} else {
		localAddressPool = ipamutils.GetDefaultLocalScopeSubnetter()
	}

	a, err := ipam.NewAllocator(localAddressPool, ipamutils.GetDefaultGlobalScopeSubnetter())
	if err != nil {
		return err
	}

	cps := &ipamapi.Capability{RequiresRequestReplay: true}

	return ic.RegisterIpamDriverWithCapabilities(ipamapi.DefaultIPAM, a, cps)
}

// SetDefaultIPAddressPool stores default address pool.
func SetDefaultIPAddressPool(addressPool []*ipamutils.NetworkToSplit) error {
	defaultAddressPool = addressPool
	return nil
}
