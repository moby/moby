package ipams

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	"github.com/docker/docker/libnetwork/ipams/defaultipam"
	"github.com/docker/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/docker/libnetwork/ipams/remote"
	"github.com/docker/docker/libnetwork/ipams/windowsipam"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/pkg/plugingetter"
)

// Register registers all the builtin drivers (ie. default, windowsipam, null
// and remote). If 'pg' is nil, the remote driver won't be registered.
func Register(r ipamapi.Registerer, pg plugingetter.PluginGetter, lAddrPools, gAddrPools []*ipamutils.NetworkToSplit) error {
	if err := defaultipam.Register(r, lAddrPools, gAddrPools); err != nil {
		return err
	}
	if err := windowsipam.Register(r); err != nil {
		return err
	}
	if err := null.Register(r); err != nil {
		return err
	}
	if pg != nil {
		if err := remoteIpam.Register(r, pg); err != nil {
			return err
		}
	}
	return nil
}
