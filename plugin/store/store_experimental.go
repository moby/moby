// +build experimental

package store

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/reference"
)

/* allowV1PluginsFallback determines daemon's support for V1 plugins.
 * When the time comes to remove support for V1 plugins, flipping
 * this bool is all that will be needed.
 */
var allowV1PluginsFallback = true

// ErrNotFound indicates that a plugin was not found locally.
type ErrNotFound string

func (name ErrNotFound) Error() string { return fmt.Sprintf("plugin %q not found", string(name)) }

// GetByName retreives a plugin by name.
func (ps *Store) GetByName(name string) (*v2.Plugin, error) {
	ps.RLock()
	defer ps.RUnlock()

	id, nameOk := ps.nameToID[name]
	if !nameOk {
		return nil, ErrNotFound(name)
	}

	p, idOk := ps.plugins[id]
	if !idOk {
		return nil, ErrNotFound(id)
	}
	return p, nil
}

// GetByID retreives a plugin by ID.
func (ps *Store) GetByID(id string) (*v2.Plugin, error) {
	ps.RLock()
	defer ps.RUnlock()

	p, idOk := ps.plugins[id]
	if !idOk {
		return nil, ErrNotFound(id)
	}
	return p, nil
}

// GetAll retreives all plugins.
func (ps *Store) GetAll() map[string]*v2.Plugin {
	ps.RLock()
	defer ps.RUnlock()
	return ps.plugins
}

// SetAll initialized plugins during daemon restore.
func (ps *Store) SetAll(plugins map[string]*v2.Plugin) {
	ps.Lock()
	defer ps.Unlock()
	ps.plugins = plugins
}

func (ps *Store) getByCap(name string, capability string) (*v2.Plugin, error) {
	ps.RLock()
	defer ps.RUnlock()

	p, err := ps.GetByName(name)
	if err != nil {
		return nil, err
	}
	return p.FilterByCap(capability)
}

func (ps *Store) getAllByCap(capability string) []plugingetter.CompatPlugin {
	ps.RLock()
	defer ps.RUnlock()

	result := make([]plugingetter.CompatPlugin, 0, 1)
	for _, p := range ps.plugins {
		if _, err := p.FilterByCap(capability); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// SetState sets the active state of the plugin and updates plugindb.
func (ps *Store) SetState(p *v2.Plugin, state bool) {
	ps.Lock()
	defer ps.Unlock()

	p.PluginObj.Enabled = state
	ps.updatePluginDB()
}

// Add adds a plugin to memory and plugindb.
func (ps *Store) Add(p *v2.Plugin) {
	ps.Lock()
	ps.plugins[p.GetID()] = p
	ps.nameToID[p.Name()] = p.GetID()
	ps.updatePluginDB()
	ps.Unlock()
}

// Remove removes a plugin from memory and plugindb.
func (ps *Store) Remove(p *v2.Plugin) {
	ps.Lock()
	delete(ps.plugins, p.GetID())
	delete(ps.nameToID, p.Name())
	ps.updatePluginDB()
	ps.Unlock()
}

// Callers are expected to hold the store lock.
func (ps *Store) updatePluginDB() error {
	jsonData, err := json.Marshal(ps.plugins)
	if err != nil {
		logrus.Debugf("Error in json.Marshal: %v", err)
		return err
	}
	ioutils.AtomicWriteFile(ps.plugindb, jsonData, 0600)
	return nil
}

// Get returns a plugin matching the given name and capability.
func (ps *Store) Get(name, capability string, mode int) (plugingetter.CompatPlugin, error) {
	var (
		p   *v2.Plugin
		err error
	)

	// Lookup using new model.
	if ps != nil {
		fullName := name
		if named, err := reference.ParseNamed(fullName); err == nil { // FIXME: validate
			if reference.IsNameOnly(named) {
				named = reference.WithDefaultTag(named)
			}
			ref, ok := named.(reference.NamedTagged)
			if !ok {
				return nil, fmt.Errorf("invalid name: %s", named.String())
			}
			fullName = ref.String()
		}
		p, err = ps.GetByName(fullName)
		if err == nil {
			p.Lock()
			p.RefCount += mode
			p.Unlock()
			return p.FilterByCap(capability)
		}
		if _, ok := err.(ErrNotFound); !ok {
			return nil, err
		}
	}

	// Lookup using legacy model.
	if allowV1PluginsFallback {
		p, err := plugins.Get(name, capability)
		if err != nil {
			return nil, fmt.Errorf("legacy plugin: %v", err)
		}
		return p, nil
	}

	return nil, err
}

// GetAllByCap returns a list of plugins matching the given capability.
func (ps *Store) GetAllByCap(capability string) ([]plugingetter.CompatPlugin, error) {
	result := make([]plugingetter.CompatPlugin, 0, 1)

	/* Daemon start always calls plugin.Init thereby initializing a store.
	 * So store on experimental builds can never be nil, even while
	 * handling legacy plugins. However, there are legacy plugin unit
	 * tests where the volume subsystem directly talks with the plugin,
	 * bypassing the daemon. For such tests, this check is necessary.
	 */
	if ps != nil {
		ps.RLock()
		result = ps.getAllByCap(capability)
		ps.RUnlock()
	}

	// Lookup with legacy model
	if allowV1PluginsFallback {
		pl, err := plugins.GetAll(capability)
		if err != nil {
			return nil, fmt.Errorf("legacy plugin: %v", err)
		}
		for _, p := range pl {
			result = append(result, p)
		}
	}
	return result, nil
}

// Handle sets a callback for a given capability. It is only used by network
// and ipam drivers during plugin registration. The callback registers the
// driver with the subsystem (network, ipam).
func (ps *Store) Handle(capability string, callback func(string, *plugins.Client)) {
	pluginType := fmt.Sprintf("docker.%s/1", strings.ToLower(capability))

	// Register callback with new plugin model.
	ps.Lock()
	ps.handlers[pluginType] = callback
	ps.Unlock()

	// Register callback with legacy plugin model.
	if allowV1PluginsFallback {
		plugins.Handle(capability, callback)
	}
}

// CallHandler calls the registered callback. It is invoked during plugin enable.
func (ps *Store) CallHandler(p *v2.Plugin) {
	for _, typ := range p.GetTypes() {
		if handler := ps.handlers[typ.String()]; handler != nil {
			handler(p.Name(), p.Client())
		}
	}
}
