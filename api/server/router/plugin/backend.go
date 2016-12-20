package plugin

import (
	"io"
	"net/http"

	enginetypes "github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

// Backend for Plugin
type Backend interface {
	Disable(name string, config *enginetypes.PluginDisableConfig) error
	Enable(name string, config *enginetypes.PluginEnableConfig) error
	List() ([]enginetypes.Plugin, error)
	Inspect(name string) (enginetypes.Plugin, error)
	Remove(name string, config *enginetypes.PluginRmConfig) error
	Set(name string, args []string) error
	Privileges(name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) (enginetypes.PluginPrivileges, error)
	Pull(name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, privileges enginetypes.PluginPrivileges) error
	Push(name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) error
	CreateFromContext(ctx context.Context, tarCtx io.Reader, options *enginetypes.PluginCreateOptions) error
}
