package libnetwork

import (
	"strings"

	"github.com/docker/libnetwork/discoverapi"
	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	"github.com/docker/libnetwork/netlabel"

	builtinIpam "github.com/docker/libnetwork/ipams/builtin"
	nullIpam "github.com/docker/libnetwork/ipams/null"
	remoteIpam "github.com/docker/libnetwork/ipams/remote"
)

type initializer struct {
	fn    func(driverapi.DriverCallback, map[string]interface{}) error
	ntype string
}

func initDrivers(c *controller) error {
	scopesConfig := make(map[string]interface{})

	for k, v := range c.cfg.Scopes {
		if v.PersistConnection() {
			scopesConfig[netlabel.MakeKVStore(k)] = c.GetStore(k)
		} else {
			if !v.IsValid() {
				continue
			}

			scopesConfig[netlabel.MakeKVClient(k)] = discoverapi.DatastoreConfigData{
				Scope:    k,
				Provider: v.Client.Provider,
				Address:  v.Client.Address,
				Config:   v.Client.Config,
			}
		}
	}

	for _, i := range getInitializers() {
		config := make(map[string]interface{})

		if drvCfg, ok := c.cfg.Daemon.DriverCfg[i.ntype]; ok {
			for k, v := range drvCfg.(map[string]interface{}) {
				config[k] = v
			}
		}

		for _, label := range c.cfg.Daemon.Labels {
			if !strings.HasPrefix(netlabel.Key(label), netlabel.DriverPrefix+"."+i.ntype) {
				continue
			}

			config[netlabel.Key(label)] = netlabel.Value(label)
		}

		// We don't send datastore configs to external plugins
		if i.ntype != "remote" {
			for sk, sv := range scopesConfig {
				config[sk] = sv
			}
		}

		if err := i.fn(c, config); err != nil {
			return err
		}
	}

	return nil
}

func initIpams(ic ipamapi.Callback, lDs, gDs interface{}) error {
	for _, fn := range [](func(ipamapi.Callback, interface{}, interface{}) error){
		builtinIpam.Init,
		remoteIpam.Init,
		nullIpam.Init,
	} {
		if err := fn(ic, lDs, gDs); err != nil {
			return err
		}
	}
	return nil
}
