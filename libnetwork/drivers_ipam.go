package libnetwork

import (
	"github.com/docker/docker/libnetwork/ipamapi"
	builtinIpam "github.com/docker/docker/libnetwork/ipams"
	nullIpam "github.com/docker/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/docker/libnetwork/ipams/remote"
	"github.com/docker/docker/libnetwork/ipamutils"
	"github.com/docker/docker/pkg/plugingetter"
)

func initIPAMDrivers(r ipamapi.Registerer, pg plugingetter.PluginGetter, addressPool []*ipamutils.NetworkToSplit) error {
	if err := builtinIpam.Register(r, addressPool); err != nil {
		return err
	}
	if err := nullIpam.Register(r); err != nil {
		return err
	}
	return remoteIpam.Register(r, pg)
}
