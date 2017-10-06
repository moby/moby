package plugin

import (
	"github.com/docker/docker/plugin/v2"
)

// Chain is the interface a type must adhere to in order to
// participate in the plugin management system's plugin chain persistence
type Chain interface {
	Type() string
	SetSaveChain(func(names []string) error)
	SetPlugins(names []string) error
	RemovePlugin(plugin *v2.Plugin) error
	AppendPluginIfMissing(plugin *v2.Plugin) error
}
