package testutils

import (
	"fmt"
	"net"

	"github.com/moby/swarmkit/v2/node/plugin"
)

const DockerCSIPluginNodeCap = "csinode"
const DockerCSIPluginControllerCap = "csicontroller"

type FakePluginGetter struct {
	Plugins map[string]*FakePlugin
}

var _ plugin.Getter = &FakePluginGetter{}

func (f *FakePluginGetter) Get(name, capability string) (plugin.Plugin, error) {
	if capability != DockerCSIPluginNodeCap && capability != DockerCSIPluginControllerCap {
		return nil, fmt.Errorf(
			"requested plugin with %s cap, but should only ever request %s or %s",
			capability, DockerCSIPluginNodeCap, DockerCSIPluginControllerCap,
		)
	}

	if plug, ok := f.Plugins[name]; ok {
		return plug, nil
	}
	return nil, fmt.Errorf("plugin %s not found", name)
}

// GetAllManagedPluginsByCap returns all of the fake's plugins. If capability
// is anything other than DockerCSIPluginCap, it returns nothing.
func (f *FakePluginGetter) GetAllManagedPluginsByCap(capability string) []plugin.Plugin {
	if capability != DockerCSIPluginNodeCap && capability != DockerCSIPluginControllerCap {
		return nil
	}

	allPlugins := make([]plugin.Plugin, 0, len(f.Plugins))
	for _, plug := range f.Plugins {
		allPlugins = append(allPlugins, plug)
	}
	return allPlugins
}

type FakePlugin struct {
	PluginName string
	PluginAddr net.Addr
	Scope      string
}

var _ plugin.AddrPlugin = &FakePlugin{}

func (f *FakePlugin) Name() string {
	return f.PluginName
}

func (f *FakePlugin) ScopedPath(path string) string {
	if f.Scope != "" {
		return fmt.Sprintf("%s/%s", f.Scope, path)
	}
	return path
}

func (f *FakePlugin) Client() plugin.Client {
	return nil
}

func (f *FakePlugin) Addr() net.Addr {
	return f.PluginAddr
}
