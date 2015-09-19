package libnetwork

import (
	"strings"

	"github.com/docker/libnetwork/driverapi"
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

	if c.validateDatastoreConfig() {
		config[netlabel.KVProvider] = c.cfg.Datastore.Client.Provider
		config[netlabel.KVProviderURL] = c.cfg.Datastore.Client.Address
	}

	for _, label := range c.cfg.Daemon.Labels {
		if !strings.HasPrefix(netlabel.Key(label), netlabel.DriverPrefix+"."+ntype) {
			continue
		}

		config[netlabel.Key(label)] = netlabel.Value(label)
	}

	drvCfg, ok := c.cfg.Daemon.DriverCfg[ntype]
	if !ok {
		return config
	}

	for k, v := range drvCfg.(map[string]interface{}) {
		config[k] = v
	}

	return config
}
