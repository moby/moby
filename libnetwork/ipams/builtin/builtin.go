package builtin

import (
	"net"

	"github.com/docker/docker/libnetwork/ipam"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
)

var (
	// defaultAddressPool Stores user configured subnet list
	defaultAddressPool []*net.IPNet
)

// registerBuiltin registers the built-in ipam driver with libnetwork.
func registerBuiltin(ic ipamapi.Registerer) error {
	var localAddressPool []*net.IPNet
	if len(defaultAddressPool) > 0 {
		localAddressPool = append([]*net.IPNet(nil), defaultAddressPool...)
	} else {
		localAddressPool = ipamutils.GetLocalScopeDefaultNetworks()
	}

	a, err := ipam.NewAllocator(localAddressPool, ipamutils.GetGlobalScopeDefaultNetworks())
	if err != nil {
		return err
	}

	cps := &ipamapi.Capability{RequiresRequestReplay: true}

	return ic.RegisterIpamDriverWithCapabilities(ipamapi.DefaultIPAM, a, cps)
}

// SetDefaultIPAddressPool stores default address pool.
func SetDefaultIPAddressPool(addressPool []*ipamutils.NetworkToSplit) error {
	nets, err := ipamutils.SplitNetworks(addressPool)
	if err != nil {
		return err
	}
	defaultAddressPool = nets
	return nil
}
