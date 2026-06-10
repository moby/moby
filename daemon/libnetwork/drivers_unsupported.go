//go:build !linux && !windows

package libnetwork

import (
	"context"

	"github.com/moby/moby/v2/daemon/libnetwork/config"
	"github.com/moby/moby/v2/daemon/libnetwork/datastore"
	"github.com/moby/moby/v2/daemon/libnetwork/driverapi"
	"github.com/moby/moby/v2/daemon/libnetwork/drvregistry"
)

func registerPortMappers(context.Context, *drvregistry.PortMappers, *config.Config) error {
	return nil
}

func registerNetworkDrivers(r driverapi.Registerer, _ *config.Config, store *datastore.Store, pms *drvregistry.PortMappers) error {
	return nil
}
