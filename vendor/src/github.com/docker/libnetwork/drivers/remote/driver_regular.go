// +build !experimental

package remote

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/driverapi"
)

func Init(dc driverapi.DriverCallback, config map[string]interface{}) error {
	plugins.Handle(driverapi.NetworkPluginEndpointType, func(name string, client *plugins.Client) {
		// negotiate driver capability with client
		d := newDriver(name, client)
		c, err := d.(*driver).getCapabilities()
		if err != nil {
			log.Errorf("error getting capability for %s due to %v", name, err)
			return
		}
		if err = dc.RegisterDriver(name, d, *c); err != nil {
			log.Errorf("error registering driver for %s due to %v", name, err)
		}
	})
	return nil
}
