//go:build linux

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package overlay

import (
	"errors"

	"github.com/moby/sys/userns"

	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/snapshots/overlay"
	"github.com/containerd/containerd/v2/plugins/snapshots/overlay/overlayutils"
	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"
)

const (
	capaRemapIDs     = "remap-ids"
	capaOnlyRemapIDs = "only-remap-ids"
	capaRebase       = "rebase"
)

// Config represents configuration for the overlay plugin.
type Config struct {
	// Root directory for the plugin
	RootPath      string `toml:"root_path"`
	UpperdirLabel bool   `toml:"upperdir_label"`
	SyncRemove    bool   `toml:"sync_remove"`

	// slowChown allows the plugin to fallback to a recursive chown if fast options (like
	// idmap mounts) are not available. See more info about the overhead this can have in
	// github.com/containerd/containerd/docs/user-namespaces/.
	SlowChown bool `toml:"slow_chown"`

	// MountOptions are options used for the overlay mount (not used on bind mounts)
	MountOptions []string `toml:"mount_options"`
}

func init() {
	registry.Register(&plugin.Registration{
		Type:   plugins.SnapshotPlugin,
		ID:     "overlayfs",
		Config: &Config{},
		InitFn: func(ic *plugin.InitContext) (any, error) {
			ic.Meta.Platforms = append(ic.Meta.Platforms, platforms.DefaultSpec())

			config, ok := ic.Config.(*Config)
			if !ok {
				return nil, errors.New("invalid overlay configuration")
			}

			root := ic.Properties[plugins.PropertyRootDir]
			if config.RootPath != "" {
				root = config.RootPath
			}

			var oOpts []overlay.Opt
			if config.UpperdirLabel {
				oOpts = append(oOpts, overlay.WithUpperdirLabel)
			}
			if !config.SyncRemove {
				oOpts = append(oOpts, overlay.AsynchronousRemove)
			}

			if len(config.MountOptions) > 0 {
				oOpts = append(oOpts, overlay.WithMountOptions(config.MountOptions))
			}
			if ok, err := overlayutils.SupportsIDMappedMounts(); err == nil && ok {
				oOpts = append(oOpts, overlay.WithRemapIDs)
				ic.Meta.Capabilities = append(ic.Meta.Capabilities, capaRemapIDs)
			}

			if config.SlowChown {
				oOpts = append(oOpts, overlay.WithSlowChown)
			} else {
				// If slowChown is false, we use capaOnlyRemapIDs to signal we only
				// allow idmap mounts.
				ic.Meta.Capabilities = append(ic.Meta.Capabilities, capaOnlyRemapIDs)
			}

			if !userns.RunningInUserNS() {
				// "rebase" capability depends on `mknod c 0 0` via OverlayConvertWhiteout,
				// so it does not work when running in UserNS.
				// https://github.com/containerd/containerd/issues/13388
				ic.Meta.Capabilities = append(ic.Meta.Capabilities, capaRebase)
			}

			ic.Meta.Exports[plugins.SnapshotterRootDir] = root
			return overlay.NewSnapshotter(root, oOpts...)
		},
	})
}
