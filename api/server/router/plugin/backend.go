// +build experimental

package plugin

import (
	"net/http"

	enginetypes "github.com/docker/docker/api/types"
)

// Backend for Plugin
type Backend interface {
	Disable(name string) error
	Enable(name string) error
	List() ([]enginetypes.Plugin, error)
	Inspect(name string) (enginetypes.Plugin, error)
	Remove(name string, config *enginetypes.PluginRmConfig) error
	Set(name string, args []string) error
	Pull(name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) (enginetypes.PluginPrivileges, error)
	Push(name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) error
}
