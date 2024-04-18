package builtin

import (
	"github.com/docker/docker/libnetwork/ipam"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
)

// registerBuiltin registers the built-in ipam driver with libnetwork. It takes
// an optional addressPools containing the list of user-defined address pools
// used by the local address space (ie. daemon's default-address-pools parameter).
func registerBuiltin(ic ipamapi.Registerer, addressPools []*ipamutils.NetworkToSplit) error {
	localAddressPools := ipamutils.GetLocalScopeDefaultNetworks()
	if len(addressPools) > 0 {
		var err error
		localAddressPools, err = ipamutils.SplitNetworks(addressPools)
		if err != nil {
			return err
		}
	}

	a, err := ipam.NewAllocator(localAddressPools, ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		return err
	}

	cps := &ipamapi.Capability{RequiresRequestReplay: true}

	return ic.RegisterIpamDriverWithCapabilities(ipamapi.DefaultIPAM, a, cps)
}
