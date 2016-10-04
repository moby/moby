package client

import (
	"io"

	"github.com/docker/docker/api/types"
	"golang.org/x/net/context"
)

type apiClientExperimental interface {
	CheckpointAPIClient
	PluginAPIClient
}

// CheckpointAPIClient defines API client methods for the checkpoints
type CheckpointAPIClient interface {
	CheckpointCreate(ctx context.Context, container string, options types.CheckpointCreateOptions) error
	CheckpointDelete(ctx context.Context, container string, options types.CheckpointDeleteOptions) error
	CheckpointList(ctx context.Context, container string, options types.CheckpointListOptions) ([]types.Checkpoint, error)
}

// PluginAPIClient defines API client methods for the plugins
type PluginAPIClient interface {
	PluginList(ctx context.Context) (types.PluginsListResponse, error)
	PluginRemove(ctx context.Context, name string, options types.PluginRemoveOptions) error
	PluginEnable(ctx context.Context, name string) error
	PluginDisable(ctx context.Context, name string) error
	PluginInstall(ctx context.Context, name string, options types.PluginInstallOptions) error
	PluginPush(ctx context.Context, name string, registryAuth string) error
	PluginSet(ctx context.Context, name string, args []string) error
	PluginInspectWithRaw(ctx context.Context, name string) (*types.Plugin, []byte, error)
	PluginCreate(ctx context.Context, createContext io.Reader, options types.PluginCreateOptions) error
}
