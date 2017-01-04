package plugingetter

import "github.com/docker/docker/pkg/plugins"

const (
	// LOOKUP doesn't update RefCount
	LOOKUP = 0
	// ACQUIRE increments RefCount
	ACQUIRE = 1
	// RELEASE decrements RefCount
	RELEASE = -1
)

// CompatPlugin is a abstraction to handle both v2(new) and v1(legacy) plugins.
type CompatPlugin interface {
	Client() *plugins.Client
	Name() string
	BasePath() string
	IsV1() bool
}

// CountedPlugin is a plugin which is reference counted.
type CountedPlugin interface {
	Acquire()
	Release()
	CompatPlugin
}

// PluginGetter is the interface implemented by Store
type PluginGetter interface {
	Get(name, capability string, mode int) (CompatPlugin, error)
	GetAllByCap(capability string) ([]CompatPlugin, error)
	GetAllManagedPluginsByCap(capability string) []CompatPlugin
	Handle(capability string, callback func(string, *plugins.Client))
}
