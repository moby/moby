//go:build !linux

package plugin

import (
	"context"
	"errors"
	"io"
	"net/http"

	"github.com/distribution/reference"
	"github.com/moby/moby/api/types/filters"
	"github.com/moby/moby/api/types/plugin"
	"github.com/moby/moby/api/types/registry"
	"github.com/moby/moby/v2/daemon/server/backend"
)

var errNotSupported = errors.New("plugins are not supported on this platform")

// Disable deactivates a plugin, which implies that they cannot be used by containers.
func (pm *Manager) Disable(name string, config *backend.PluginDisableConfig) error {
	return errNotSupported
}

// Enable activates a plugin, which implies that they are ready to be used by containers.
func (pm *Manager) Enable(name string, config *backend.PluginEnableConfig) error {
	return errNotSupported
}

// Inspect examines a plugin config
func (pm *Manager) Inspect(refOrID string) (*plugin.Plugin, error) {
	return nil, errNotSupported
}

// Privileges pulls a plugin config and computes the privileges required to install it.
func (pm *Manager) Privileges(ctx context.Context, ref reference.Named, metaHeader http.Header, authConfig *registry.AuthConfig) (plugin.Privileges, error) {
	return nil, errNotSupported
}

// Pull pulls a plugin, check if the correct privileges are provided and install the plugin.
func (pm *Manager) Pull(ctx context.Context, ref reference.Named, name string, metaHeader http.Header, authConfig *registry.AuthConfig, privileges plugin.Privileges, out io.Writer, opts ...CreateOpt) error {
	return errNotSupported
}

// Upgrade pulls a plugin, check if the correct privileges are provided and install the plugin.
func (pm *Manager) Upgrade(ctx context.Context, ref reference.Named, name string, metaHeader http.Header, authConfig *registry.AuthConfig, privileges plugin.Privileges, outStream io.Writer) error {
	return errNotSupported
}

// List displays the list of plugins and associated metadata.
func (pm *Manager) List(pluginFilters filters.Args) ([]plugin.Plugin, error) {
	return nil, errNotSupported
}

// Push pushes a plugin to the store.
func (pm *Manager) Push(ctx context.Context, name string, metaHeader http.Header, authConfig *registry.AuthConfig, out io.Writer) error {
	return errNotSupported
}

// Remove deletes plugin's root directory.
func (pm *Manager) Remove(name string, config *backend.PluginRmConfig) error {
	return errNotSupported
}

// Set sets plugin args
func (pm *Manager) Set(name string, args []string) error {
	return errNotSupported
}

// CreateFromContext creates a plugin from the given pluginDir which contains
// both the rootfs and the config.json and a repoName with optional tag.
func (pm *Manager) CreateFromContext(ctx context.Context, tarCtx io.ReadCloser, options *backend.PluginCreateConfig) error {
	return errNotSupported
}
