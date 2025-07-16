package ipams

import (
	"github.com/moby/moby/v2/daemon/libnetwork/ipamapi"
	"github.com/moby/moby/v2/daemon/libnetwork/ipams/defaultipam"
	"github.com/moby/moby/v2/daemon/libnetwork/ipams/null"
	remoteIpam "github.com/moby/moby/v2/daemon/libnetwork/ipams/remote"
	"github.com/moby/moby/v2/daemon/libnetwork/ipamutils"
	"github.com/moby/moby/v2/pkg/plugingetter"
)

// Register registers all the builtin drivers (ie. default, null
// and remote). 'pg' is nil here in case of non-managed plugins which Windows is using.
func Register(r ipamapi.Registerer, pg plugingetter.PluginGetter, lAddrPools, gAddrPools []*ipamutils.NetworkToSplit) error {
	if err := defaultipam.Register(r, lAddrPools, gAddrPools); err != nil {
		return err
	}
	if err := null.Register(r); err != nil {
		return err
	}
	if err := remoteIpam.Register(r, pg); err != nil {
		return err
	}
	return nil
}
