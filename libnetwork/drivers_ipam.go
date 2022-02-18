package libnetwork

import (
	"github.com/moby/moby/libnetwork/drvregistry"
	"github.com/moby/moby/libnetwork/ipamapi"
	builtinIpam "github.com/moby/moby/libnetwork/ipams/builtin"
	nullIpam "github.com/moby/moby/libnetwork/ipams/null"
	remoteIpam "github.com/moby/moby/libnetwork/ipams/remote"
	"github.com/moby/moby/libnetwork/ipamutils"
)

func initIPAMDrivers(r *drvregistry.DrvRegistry, lDs, gDs interface{}, addressPool []*ipamutils.NetworkToSplit) error {
	builtinIpam.SetDefaultIPAddressPool(addressPool)
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
