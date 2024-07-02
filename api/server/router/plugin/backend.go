package plugin // import "github.com/docker/docker/api/server/router/plugin"

import (
	"context"
	"io"
	"net/http"

	"github.com/distribution/reference"
	"github.com/docker/docker/api/types/backend"
	"github.com/docker/docker/api/types/filters"
	plugintypes "github.com/docker/docker/api/types/plugin"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/plugin"
)

// Backend for Plugin
type Backend interface {
	Disable(name string, config *backend.PluginDisableConfig) error
	Enable(name string, config *backend.PluginEnableConfig) error
	List(filters.Args) ([]plugintypes.Plugin, error)
	Inspect(name string) (*plugintypes.Plugin, error)
	Remove(name string, config *backend.PluginRmConfig) error
	Set(name string, args []string) error
	Privileges(ctx context.Context, ref reference.Named, metaHeaders http.Header, authConfig *registry.AuthConfig) (plugintypes.Privileges, error)
	Pull(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *registry.AuthConfig, privileges plugintypes.Privileges, outStream io.Writer, opts ...plugin.CreateOpt) error
	Push(ctx context.Context, name string, metaHeaders http.Header, authConfig *registry.AuthConfig, outStream io.Writer) error
	Upgrade(ctx context.Context, ref reference.Named, name string, metaHeaders http.Header, authConfig *registry.AuthConfig, privileges plugintypes.Privileges, outStream io.Writer) error
	CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *plugintypes.CreateOptions) error
}
