// +build experimental

package authorization

import (
	pluginstore "github.com/docker/docker/plugin/store"
)

// initPlugin initializes the authorization plugin if needed.
func (a *authorizationPlugin) initPlugin() error {
	var err error
	a.once.Do(func() {
		if a.plugin == nil {
			plugin, e := pluginstore.LookupWithCapability(a.name, AuthZApiImplements)
			if e != nil {
				err = e
				return
			}
			a.plugin = plugin.Client()
		}
	})
	return err
}
