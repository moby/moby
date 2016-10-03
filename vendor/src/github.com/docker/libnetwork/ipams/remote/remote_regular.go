// +build !experimental

package remote

import (
	log "github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/libnetwork/ipamapi"
)

// Init registers a remote ipam when its plugin is activated
func Init(cb ipamapi.Callback, l, g interface{}) error {
	plugins.Handle(ipamapi.PluginEndpointType, func(name string, client *plugins.Client) {
		a := newAllocator(name, client)
		if cps, err := a.(*allocator).getCapabilities(); err == nil {
			if err := cb.RegisterIpamDriverWithCapabilities(name, a, cps); err != nil {
				log.Errorf("error registering remote ipam driver %s due to %v", name, err)
			}
		} else {
			log.Infof("remote ipam driver %s does not support capabilities", name)
			log.Debug(err)
			if err := cb.RegisterIpamDriver(name, a); err != nil {
				log.Errorf("error registering remote ipam driver %s due to %v", name, err)
			}
		}
	})
	return nil
}
