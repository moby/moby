// +build !experimental

package graphdriver

import "github.com/docker/docker/plugin/getter"

func lookupPlugin(name, home string, opts []string, plugingetter getter.PluginGetter) (Driver, error) {
	return nil, ErrNotSupported
}
