//go:build (linux || windows) && !no_embedded_containerd

package embedded

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"slices"

	srvconfig "github.com/containerd/containerd/v2/cmd/containerd/server/config"
	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
	"github.com/moby/moby/v2/daemon/internal/containerd/server/dropin"
)

// loadConfigDropIns loads the containerd configuration drop-in files in dir,
// migrating files with older configuration versions like containerd does.
func loadConfigDropIns(ctx context.Context, dir string) (srvconfig.Config, error) {
	files, err := dropin.Files(dir)
	if err != nil {
		return srvconfig.Config{}, err
	}

	registrations := func() iter.Seq[plugin.Registration] {
		return slices.Values(registry.Graph(nil))
	}
	cfg := srvconfig.Config{Version: version.ConfigVersion}
	for _, path := range files {
		if err := srvconfig.LoadConfigWithPlugins(ctx, path, registrations, &cfg); err != nil {
			return srvconfig.Config{}, fmt.Errorf("failed to load containerd config drop-in %q: %w", path, err)
		}
	}

	// dockerd configures the embedded server itself, and does not implement
	// containerd's process and server settings.
	ignored := cfg
	ignored.Version = 0
	ignored.Plugins = nil
	ignored.DisabledPlugins = nil
	ignored.RequiredPlugins = nil
	if !reflect.ValueOf(ignored).IsZero() {
		log.G(ctx).WithField("dir", dir).Warn("embedded containerd only supports the plugins, disabled_plugins, and required_plugins settings in config drop-ins, ignoring other settings")
	}
	return cfg, nil
}
