package plugin

import "github.com/docker/docker/pkg/plugins"

// Plugin represents a plugin. It is used to abstract from an older plugin architecture (in pkg/plugins).
type Plugin interface {
	Client() *plugins.Client
	Name() string
	IsLegacy() bool
}
