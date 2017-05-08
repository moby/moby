// +build experimental

package store

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/docker/docker/pkg/ioutils"
	"github.com/docker/docker/pkg/plugins"
	"github.com/docker/docker/plugin/v2"
	"github.com/docker/docker/reference"
)

var (
	store *PluginStore
	/* allowV1PluginsFallback determines daemon's support for V1 plugins.
	 * When the time comes to remove support for V1 plugins, flipping
	 * this bool is all that will be needed.
	 */
	allowV1PluginsFallback = true
)

// ErrNotFound indicates that a plugin was not found locally.
type ErrNotFound string

func (name ErrNotFound) Error() string { return fmt.Sprintf("plugin %q not found", string(name)) }

// PluginStore manages the plugin inventory in memory and on-disk
type PluginStore struct {
	sync.RWMutex
	plugins  map[string]*v2.Plugin
	nameToID map[string]string
	plugindb string
}

// NewPluginStore creates a PluginStore.
func NewPluginStore(libRoot string) *PluginStore {
	store = &PluginStore{
		plugins:  make(map[string]*v2.Plugin),
		nameToID: make(map[string]string),
		plugindb: filepath.Join(libRoot, "plugins.json"),
	}
	return store
}

// GetByName retreives a plugin by name.
func (ps *PluginStore) GetByName(name string) (*v2.Plugin, error) {
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
func (ps *PluginStore) GetByID(id string) (*v2.Plugin, error) {
	ps.RLock()
	defer ps.RUnlock()

	p, idOk := ps.plugins[id]
	if !idOk {
		return nil, ErrNotFound(id)
	}
	return p, nil
}

// GetAll retreives all plugins.
func (ps *PluginStore) GetAll() map[string]*v2.Plugin {
	ps.RLock()
	defer ps.RUnlock()
	return ps.plugins
}

// SetAll initialized plugins during daemon restore.
func (ps *PluginStore) SetAll(plugins map[string]*v2.Plugin) {
	ps.Lock()
	defer ps.Unlock()
	ps.plugins = plugins
}

func (ps *PluginStore) getByCap(name string, capability string) (*v2.Plugin, error) {
	ps.RLock()
	defer ps.RUnlock()

	p, err := ps.GetByName(name)
	if err != nil {
		return nil, err
	}
	return p.FilterByCap(capability)
}

func (ps *PluginStore) getAllByCap(capability string) []CompatPlugin {
	ps.RLock()
	defer ps.RUnlock()

	result := make([]CompatPlugin, 0, 1)
	for _, p := range ps.plugins {
		if _, err := p.FilterByCap(capability); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// SetState sets the active state of the plugin and updates plugindb.
func (ps *PluginStore) SetState(p *v2.Plugin, state bool) {
	ps.Lock()
	defer ps.Unlock()

	p.PluginObj.Enabled = state
	ps.updatePluginDB()
}

// Add adds a plugin to memory and plugindb.
func (ps *PluginStore) Add(p *v2.Plugin) {
	ps.Lock()
	ps.plugins[p.GetID()] = p
	ps.nameToID[p.Name()] = p.GetID()
	ps.updatePluginDB()
	ps.Unlock()
}

// Remove removes a plugin from memory, plugindb and disk.
func (ps *PluginStore) Remove(p *v2.Plugin) {
	ps.Lock()
	delete(ps.plugins, p.GetID())
	delete(ps.nameToID, p.Name())
	ps.updatePluginDB()
	p.RemoveFromDisk()
	ps.Unlock()
}

// Callers are expected to hold the store lock.
func (ps *PluginStore) updatePluginDB() error {
	jsonData, err := json.Marshal(ps.plugins)
	if err != nil {
		logrus.Debugf("Error in json.Marshal: %v", err)
		return err
	}
	ioutils.AtomicWriteFile(ps.plugindb, jsonData, 0600)
	return nil
}

// LookupWithCapability returns a plugin matching the given name and capability.
func LookupWithCapability(name, capability string) (CompatPlugin, error) {
	var (
		p   *v2.Plugin
		err error
	)

	// Lookup using new model.
	if store != nil {
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
		p, err = store.GetByName(fullName)
		if err == nil {
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

// FindWithCapability returns a list of plugins matching the given capability.
func FindWithCapability(capability string) ([]CompatPlugin, error) {
	result := make([]CompatPlugin, 0, 1)

	/* Daemon start always calls plugin.Init thereby initializing a store.
	 * So store on experimental builds can never be nil, even while
	 * handling legacy plugins. However, there are legacy plugin unit
	 * tests where the volume subsystem directly talks with the plugin,
	 * bypassing the daemon. For such tests, this check is necessary.
	 */
	if store != nil {
		store.RLock()
		result = store.getAllByCap(capability)
		store.RUnlock()
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
