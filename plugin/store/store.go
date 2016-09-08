// +build !experimental

package store

import (
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/getter"
)

// GetAllByCap returns a list of plugins matching the given capability.
func (ps Store) GetAllByCap(capability string) ([]getter.CompatPlugin, error) {
	pl, err := plugins.GetAll(capability)
	if err != nil {
		return nil, err
	}
	result := make([]getter.CompatPlugin, len(pl))
	for i, p := range pl {
		result[i] = p
	}
	return result, nil
}

// Get returns a plugin matching the given name and capability.
func (ps Store) Get(name, capability string, _ int) (getter.CompatPlugin, error) {
	return plugins.Get(name, capability)
}
