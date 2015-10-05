package libnetwork

import (
	"strings"

	"github.com/docker/libnetwork/driverapi"
	"github.com/docker/libnetwork/ipamapi"
	builtinIpam "github.com/docker/libnetwork/ipams/builtin"
	remoteIpam "github.com/docker/libnetwork/ipams/remote"
	"github.com/docker/libnetwork/netlabel"
)

type initializer struct {
	fn    func(driverapi.DriverCallback, map[string]interface{}) error
	ntype string
}

func initDrivers(c *controller) error {
	for _, i := range getInitializers() {
		if err := i.fn(c, makeDriverConfig(c, i.ntype)); err != nil {
			return err
		}
	}

	return nil
}

func makeDriverConfig(c *controller, ntype string) map[string]interface{} {
	if c.cfg == nil {
		return nil
	}

	config := make(map[string]interface{})

	for _, label := range c.cfg.Daemon.Labels {
		if !strings.HasPrefix(netlabel.Key(label), netlabel.DriverPrefix+"."+ntype) {
			continue
		}

		config[netlabel.Key(label)] = netlabel.Value(label)
	}

	drvCfg, ok := c.cfg.Daemon.DriverCfg[ntype]
	if ok {
		for k, v := range drvCfg.(map[string]interface{}) {
			config[k] = v
		}
	}

	// We don't send datastore configs to external plugins
	if ntype == "remote" {
		return config
	}

	for k, v := range c.cfg.Scopes {
		if !v.IsValid() {
			continue
		}

		config[netlabel.MakeKVProvider(k)] = v.Client.Provider
		config[netlabel.MakeKVProviderURL(k)] = v.Client.Address
		config[netlabel.MakeKVProviderConfig(k)] = v.Client.Config
	}

	return config
}

func initIpams(ic ipamapi.Callback, lDs, gDs interface{}) error {
	for _, fn := range [](func(ipamapi.Callback, interface{}, interface{}) error){
		builtinIpam.Init,
		remoteIpam.Init,
	} {
		if err := fn(ic, lDs, gDs); err != nil {
			return err
		}
	}
	return nil
}
