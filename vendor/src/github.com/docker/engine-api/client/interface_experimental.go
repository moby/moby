// +build experimental

package client

import (
	"io"

	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

// APIClient is an interface that clients that talk with a docker server must implement.
type APIClient interface {
	CommonAPIClient
	PluginList(ctx context.Context) (types.PluginsListResponse, error)
	PluginRemove(ctx context.Context, name string) error
	PluginEnable(ctx context.Context, name string) error
	PluginDisable(ctx context.Context, name string) error
	PluginInstall(ctx context.Context, name, registryAuth string, acceptAllPermissions, noEnable bool, in io.ReadCloser, out io.Writer) error
	PluginPush(ctx context.Context, name string, registryAuth string) error
	PluginSet(ctx context.Context, name string, args []string) error
	PluginInspect(ctx context.Context, name string) (*types.Plugin, error)
}

// Ensure that Client always implements APIClient.
var _ APIClient = &Client{}
