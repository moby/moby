package plugin // import "github.com/docker/docker/api/server/router/plugin"

import (
	"context"
	"io"
	"net/http"

	"github.com/docker/distribution/reference"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/plugin"
)

// Backend for Plugin
type Backend interface {
	Disable(ctx context.Context, name string, config *enginetypes.PluginDisableConfig) error
	Enable(ctx context.Context, name string, config *enginetypes.PluginEnableConfig) error
	List(context.Context, filters.Args) ([]enginetypes.Plugin, error)
	Inspect(ctx context.Context, name string) (*enginetypes.Plugin, error)
	Remove(ctx context.Context, name string, config *enginetypes.PluginRmConfig) error
	Set(ctx context.Context, name string, args []string) error
	Privileges(ctx context.Context, ref reference.Named, metaHeaders http.Header, authConfig *enginetypes.AuthConfig) (enginetypes.PluginPrivileges, error)
	Pull(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, privileges enginetypes.PluginPrivileges, outStream io.Writer, opts ...plugin.CreateOpt) error
	Push(ctx context.Context, name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, outStream io.Writer) error
	Upgrade(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *enginetypes.AuthConfig, privileges enginetypes.PluginPrivileges, outStream io.Writer) error
	CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *enginetypes.PluginCreateOptions) error
}
