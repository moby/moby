package plugin

import "net"

type Plugin interface {
	Name() string
	ScopedPath(string) string
	Client() Client
}

type AddrPlugin interface {
	Plugin
	Addr() net.Addr
}

type Client interface {
	Call(method string, args, ret any) error
}

type Getter interface {
	Get(name, capability string) (Plugin, error)
	GetAllManagedPluginsByCap(capability string) []Plugin
}
