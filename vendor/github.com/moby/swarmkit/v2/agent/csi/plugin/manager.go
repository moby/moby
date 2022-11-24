package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/docker/docker/pkg/plugingetter"

	"github.com/moby/swarmkit/v2/api"
)

const (
	// DockerCSIPluginCap is the capability name of the plugins we use with the
	// PluginGetter to get only the plugins we need. The full name of the
	// plugin interface is "docker.csinode/1.0". This gets only plugins with
	// Node capabilities.
	DockerCSIPluginCap = "csinode"
)

// Manager manages the multiple CSI plugins that may be in use on the
// node. Manager should be thread-safe.
type Manager interface {
	// Get gets the plugin with the given name
	Get(name string) (NodePlugin, error)

	// NodeInfo returns the NodeCSIInfo for every active plugin.
	NodeInfo(ctx context.Context) ([]*api.NodeCSIInfo, error)
}

type pluginManager struct {
	plugins   map[string]NodePlugin
	pluginsMu sync.Mutex

	// newNodePluginFunc usually points to NewNodePlugin. However, for testing,
	// NewNodePlugin can be swapped out with a function that creates fake node
	// plugins
	newNodePluginFunc func(string, plugingetter.CompatPlugin, plugingetter.PluginAddr, SecretGetter) NodePlugin

	// secrets is a SecretGetter for use by node plugins.
	secrets SecretGetter

	pg plugingetter.PluginGetter
}

func NewManager(pg plugingetter.PluginGetter, secrets SecretGetter) Manager {
	return &pluginManager{
		plugins:           map[string]NodePlugin{},
		newNodePluginFunc: NewNodePlugin,
		secrets:           secrets,
		pg:                pg,
	}
}

func (pm *pluginManager) Get(name string) (NodePlugin, error) {
	pm.pluginsMu.Lock()
	defer pm.pluginsMu.Unlock()

	plugin, err := pm.getPlugin(name)
	if err != nil {
		return nil, fmt.Errorf("cannot get plugin %v: %v", name, err)
	}

	return plugin, nil
}

func (pm *pluginManager) NodeInfo(ctx context.Context) ([]*api.NodeCSIInfo, error) {
	// TODO(dperny): do not acquire this lock for the duration of the the
	// function call. that's too long and too blocking.
	pm.pluginsMu.Lock()
	defer pm.pluginsMu.Unlock()

	// first, we should make sure all of the plugins are initialized. do this
	// by looking up all the current plugins with DockerCSIPluginCap.
	plugins := pm.pg.GetAllManagedPluginsByCap(DockerCSIPluginCap)
	for _, plugin := range plugins {
		// TODO(dperny): use this opportunity to drop plugins that we're
		// tracking but which no longer exist.

		// we don't actually need the plugin returned, we just need it loaded
		// as a side effect.
		pm.getPlugin(plugin.Name())
	}

	nodeInfo := []*api.NodeCSIInfo{}
	for _, plugin := range pm.plugins {
		info, err := plugin.NodeGetInfo(ctx)
		if err != nil {
			// skip any plugin that returns an error
			continue
		}

		nodeInfo = append(nodeInfo, info)
	}
	return nodeInfo, nil
}

// getPlugin looks up the plugin with the specified name. Loads the plugin if
// not yet loaded.
//
// pm.pluginsMu must be obtained before calling this method.
func (pm *pluginManager) getPlugin(name string) (NodePlugin, error) {
	if p, ok := pm.plugins[name]; ok {
		return p, nil
	}

	pc, err := pm.pg.Get(name, DockerCSIPluginCap, plugingetter.Lookup)
	if err != nil {
		return nil, err
	}

	pa, ok := pc.(plugingetter.PluginAddr)
	if !ok {
		return nil, fmt.Errorf("plugin does not implement PluginAddr interface")
	}

	p := pm.newNodePluginFunc(name, pc, pa, pm.secrets)
	pm.plugins[name] = p
	return p, nil
}
