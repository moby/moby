package plugins

import (
	"sync"

	"github.com/docker/docker/pkg/tlsconfig"
)

// Manifest lists what a plugin implements.
type Manifest struct {
	// List of subsystem the plugin implements.
	Implements []string
}

// Plugin is the definition of a docker plugin.
type Plugin struct {
	// Name of the plugin
	Name string `json:"-"`
	// Address of the plugin
	Addr string
	// TLS configuration of the plugin
	TLSConfig tlsconfig.Options
	// Client attached to the plugin
	Client *Client `json:"-"`
	// Manifest of the plugin (see above)
	Manifest *Manifest `json:"-"`

	activatErr   error
	activateOnce sync.Once
}
