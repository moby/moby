// +build !experimental

package plugin

import "github.com/docker/docker/pkg/plugins"

// FindWithCapability returns a list of plugins matching the given capability.
func FindWithCapability(capability string) ([]Plugin, error) {
	pl, err := plugins.GetAll(capability)
	if err != nil {
		return nil, err
	}
	result := make([]Plugin, len(pl))
	for i, p := range pl {
		result[i] = p
	}
	return result, nil
}

// LookupWithCapability returns a plugin matching the given name and capability.
func LookupWithCapability(name, capability string) (Plugin, error) {
	return plugins.Get(name, capability)
}
