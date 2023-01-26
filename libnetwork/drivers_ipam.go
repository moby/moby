package libnetwork

import (
	"github.com/docker/docker/libnetwork/drvregistry"
	"github.com/docker/docker/libnetwork/ipamapi"
	builtinIpam "github.com/docker/docker/libnetwork/ipams/builtin"
	nullIpam "github.com/docker/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/docker/libnetwork/ipams/remote"
	"github.com/docker/docker/libnetwork/ipamutils"
)

func initIPAMDrivers(r *drvregistry.DrvRegistry, lDs, gDs interface{}, addressPool []*ipamutils.NetworkToSplit) error {
	// TODO: pass address pools as arguments to builtinIpam.Init instead of
	// indirectly through global mutable state. Swarmkit references that
	// function so changing its signature breaks the build.
	if err := builtinIpam.SetDefaultIPAddressPool(addressPool); err != nil {
		return err
	}
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
