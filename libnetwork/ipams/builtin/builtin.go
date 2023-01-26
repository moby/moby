package builtin

import (
	"errors"
	"net"

	"github.com/docker/docker/libnetwork/datastore"
	"github.com/docker/docker/libnetwork/ipam"
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipamutils"
)

var (
	// defaultAddressPool Stores user configured subnet list
	defaultAddressPool []*net.IPNet
)

// initBuiltin registers the built-in ipam service with libnetwork
func initBuiltin(ic ipamapi.Callback, l, g interface{}) error {
	var (
		ok                bool
		localDs, globalDs datastore.DataStore
	)

	if l != nil {
		if localDs, ok = l.(datastore.DataStore); !ok {
			return errors.New("incorrect local datastore passed to built-in ipam init")
		}
	}

	if g != nil {
		if globalDs, ok = g.(datastore.DataStore); !ok {
			return errors.New("incorrect global datastore passed to built-in ipam init")
		}
	}

	var localAddressPool []*net.IPNet
	if len(defaultAddressPool) > 0 {
		localAddressPool = append([]*net.IPNet(nil), defaultAddressPool...)
	} else {
		localAddressPool = ipamutils.GetLocalScopeDefaultNetworks()
	}

	a, err := ipam.NewAllocator(localDs, globalDs, localAddressPool, ipamutils.GetGlobalScopeDefaultNetworks())
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
