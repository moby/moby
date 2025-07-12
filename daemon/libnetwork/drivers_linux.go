package libnetwork

import (
	"context"
	"fmt"

	"github.com/docker/docker/daemon/libnetwork/config"
	"github.com/docker/docker/daemon/libnetwork/datastore"
	"github.com/docker/docker/daemon/libnetwork/driverapi"
	"github.com/docker/docker/daemon/libnetwork/drivers/bridge"
	"github.com/docker/docker/daemon/libnetwork/drivers/host"
	"github.com/docker/docker/daemon/libnetwork/drivers/ipvlan"
	"github.com/docker/docker/daemon/libnetwork/drivers/macvlan"
	"github.com/docker/docker/daemon/libnetwork/drivers/null"
	"github.com/docker/docker/daemon/libnetwork/drivers/overlay"
	"github.com/docker/docker/daemon/libnetwork/drvregistry"
	"github.com/docker/docker/daemon/libnetwork/internal/rlkclient"
	"github.com/docker/docker/daemon/libnetwork/portmappers/nat"
	"github.com/docker/docker/daemon/libnetwork/portmappers/proxy"
	"github.com/docker/docker/daemon/libnetwork/portmappers/routed"
)

func registerNetworkDrivers(r driverapi.Registerer, store *datastore.Store, pms *drvregistry.PortMappers, driverConfig func(string) map[string]interface{}) error {
	for _, nr := range []struct {
		ntype    string
		register func(driverapi.Registerer, *datastore.Store, map[string]interface{}) error
	}{
		{ntype: bridge.NetworkType, register: func(r driverapi.Registerer, store *datastore.Store, cfg map[string]interface{}) error {
			return bridge.Register(r, store, pms, cfg)
		}},
		{ntype: host.NetworkType, register: func(r driverapi.Registerer, _ *datastore.Store, _ map[string]interface{}) error {
			return host.Register(r)
		}},
		{ntype: ipvlan.NetworkType, register: ipvlan.Register},
		{ntype: macvlan.NetworkType, register: macvlan.Register},
		{ntype: null.NetworkType, register: func(r driverapi.Registerer, _ *datastore.Store, _ map[string]interface{}) error {
			return null.Register(r)
		}},
		{ntype: overlay.NetworkType, register: func(r driverapi.Registerer, _ *datastore.Store, config map[string]interface{}) error {
			return overlay.Register(r, config)
		}},
	} {
		if err := nr.register(r, store, driverConfig(nr.ntype)); err != nil {
			return fmt.Errorf("failed to register %q driver: %w", nr.ntype, err)
		}
	}

	return nil
}

func registerPortMappers(ctx context.Context, r *drvregistry.PortMappers, cfg *config.Config) error {
	var pdc *rlkclient.PortDriverClient
	if cfg.Rootless {
		var err error
		pdc, err = rlkclient.NewPortDriverClient(ctx)
		if err != nil {
			return fmt.Errorf("failed to create port driver client: %w", err)
		}
	}

	proxyMgr := proxy.ProxyManager{ProxyPath: cfg.UserlandProxyPath}

	if err := nat.Register(r, nat.Config{
		RlkClient:    pdc,
		ProxyManager: proxyMgr,
		EnableProxy:  cfg.EnableUserlandProxy && cfg.UserlandProxyPath != "",
	}); err != nil {
		return fmt.Errorf("registering nat portmapper: %w", err)
	}

	if err := routed.Register(r); err != nil {
		return fmt.Errorf("registering routed portmapper: %w", err)
	}

	if err := proxy.Register(r, proxyMgr); err != nil {
		return fmt.Errorf("registering proxy portmapper: %w", err)
	}

	return nil
}
