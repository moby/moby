package plugingetter

import "github.com/docker/docker/pkg/plugins"

const (
	// LOOKUP doesn't update RefCount
	LOOKUP = 0
	// CREATE increments RefCount
	CREATE = 1
	// REMOVE decrements RefCount
	REMOVE = -1
)

// CompatPlugin is a abstraction to handle both v2(new) and v1(legacy) plugins.
type CompatPlugin interface {
	Client() *plugins.Client
	Name() string
	IsV1() bool
}

// PluginGetter is the interface implemented by Store
type PluginGetter interface {
	Get(name, capability string, mode int) (CompatPlugin, error)
	GetAllByCap(capability string) ([]CompatPlugin, error)
	Handle(capability string, callback func(string, *plugins.Client))
}
