//+build !test

package mountpoint

import (
	"github.com/docker/docker/pkg/plugingetter"
	"github.com/docker/docker/pkg/plugins"
)

// Plugin is a type of Middleware that interposes file system mount
// points with operation occurring out of process
type Plugin interface {
	Middleware
}

var pluginCache map[string]Plugin

// NewPlugins constructs and initializes the mount point plugins based
// on plugin names
func NewPlugins(names []string) ([]Plugin, error) {
	plugins := []Plugin{}
	pluginsMap := make(map[string]struct{})
	for _, name := range names {
		if _, ok := pluginsMap[name]; ok {
			continue
		}
		pluginsMap[name] = struct{}{}
		plugin, err := NewMountPointPlugin(name)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, plugin)
	}
	return plugins, nil
}

var getter plugingetter.PluginGetter

// SetPluginGetter sets the plugingetter
func SetPluginGetter(pg plugingetter.PluginGetter) {
	getter = pg
}

// GetPluginGetter gets the plugingetter
func GetPluginGetter() plugingetter.PluginGetter {
	return getter
}

// mountPointPlugin is an internal adapter to the docker plugin system
// and implements the Middleware interface
type mountPointPlugin struct {
	plugin   *plugins.Client
	name     string
	patterns []Pattern
}

// NewMountPointPlugin of a name will return a plugin object or an
// error. The plugin may be created new and involve a plugin
// properties query or it may come from a cache of already initialized
// mount point plugin objects.
func NewMountPointPlugin(name string) (Plugin, error) {
	var e error
	var plugin plugingetter.CompatPlugin

	if pluginCache == nil {
		pluginCache = make(map[string]Plugin)
	}

	if plugin, ok := pluginCache[name]; ok {
		return plugin, nil
	}

	if pg := GetPluginGetter(); pg != nil {
		plugin, e = pg.Get(name, MountPointAPIImplements, plugingetter.Lookup)
	} else {
		plugin, e = plugins.Get(name, MountPointAPIImplements)
	}
	if e != nil {
		return nil, e
	}

	var properties *PropertiesResponse
	properties, e = mountPointPropertiesInitialized(plugin.Client(), &PropertiesRequest{})
	if e != nil {
		return nil, e
	}

	mpp := &mountPointPlugin{
		plugin:   plugin.Client(),
		name:     plugin.Name(),
		patterns: properties.Patterns,
	}
	pluginCache[name] = mpp
	return mpp, nil
}

func (b *mountPointPlugin) Name() string {
	return "plugin:" + b.name
}

func (b *mountPointPlugin) PluginName() string {
	return b.name
}

func (b *mountPointPlugin) Patterns() []Pattern {
	return b.patterns
}

func (b *mountPointPlugin) Destroy() {
	delete(pluginCache, b.name)
}

func (b *mountPointPlugin) MountPointAttach(req *AttachRequest) (*AttachResponse, error) {
	res := &AttachResponse{}
	if err := b.plugin.Call(MountPointAPIAttach, req, res); err != nil {
		return nil, err
	}

	return res, nil
}

func (b *mountPointPlugin) MountPointDetach(req *DetachRequest) (*DetachResponse, error) {
	res := &DetachResponse{}
	if err := b.plugin.Call(MountPointAPIDetach, req, res); err != nil {
		return nil, err
	}

	return res, nil
}

func (b *mountPointPlugin) MountPointProperties(req *PropertiesRequest) (*PropertiesResponse, error) {
	return mountPointPropertiesInitialized(b.plugin, req)
}

// mountPointPropertiesInitialized performs a mount point plugin properties request on a plugin client
func mountPointPropertiesInitialized(plugin *plugins.Client, req *PropertiesRequest) (*PropertiesResponse, error) {
	res := &PropertiesResponse{}
	if err := plugin.Call(MountPointAPIProperties, req, res); err != nil {
		return nil, err
	}

	return res, nil
}
