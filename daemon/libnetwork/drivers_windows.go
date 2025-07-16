package libnetwork

import (
	"context"
	"fmt"

	"github.com/docker/docker/daemon/libnetwork/config"
	"github.com/docker/docker/daemon/libnetwork/datastore"
	"github.com/docker/docker/daemon/libnetwork/driverapi"
	"github.com/docker/docker/daemon/libnetwork/drivers/null"
	"github.com/docker/docker/daemon/libnetwork/drivers/windows"
	"github.com/docker/docker/daemon/libnetwork/drivers/windows/overlay"
	"github.com/docker/docker/daemon/libnetwork/drvregistry"
)

func registerNetworkDrivers(r driverapi.Registerer, store *datastore.Store, _ *drvregistry.PortMappers, _ func(string) map[string]interface{}) error {
	for _, nr := range []struct {
		ntype    string
		register func(driverapi.Registerer) error
	}{
		{ntype: null.NetworkType, register: null.Register},
		{ntype: overlay.NetworkType, register: overlay.Register},
	} {
		if err := nr.register(r); err != nil {
			return fmt.Errorf("failed to register %q driver: %w", nr.ntype, err)
		}
	}

	return windows.RegisterBuiltinLocalDrivers(r, store)
}

func registerPortMappers(ctx context.Context, r *drvregistry.PortMappers, cfg *config.Config) error {
	return nil
}
