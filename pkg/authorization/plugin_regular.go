// +build !experimental

package authorization

import (
	"github.com/docker/docker/pkg/plugins"
)

// initPlugin initializes the authorization plugin if needed.
func (a *authorizationPlugin) initPlugin() error {
	var err error
	a.once.Do(func() {
		if a.plugin == nil {
			plugin, e := plugins.Get(a.name, AuthZApiImplements)
			if e != nil {
				err = e
				return
			}
			a.plugin = plugin.Client()
		}
	})
	return err
}
