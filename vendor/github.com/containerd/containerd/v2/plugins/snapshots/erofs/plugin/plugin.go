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

package plugin

import (
	"errors"
	"fmt"

	"github.com/containerd/platforms"
	"github.com/containerd/plugin"
	"github.com/containerd/plugin/registry"

	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/plugins"
	"github.com/containerd/containerd/v2/plugins/snapshots/erofs"
	"github.com/docker/go-units"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	capaRemapIDs     = "remap-ids"
	capaOnlyRemapIDs = "only-remap-ids"
)

// Config represents configuration for the native plugin.
type Config struct {
	// Root directory for the plugin
	RootPath string `toml:"root_path"`

	// MountOptions are options used for the EROFS overlayfs mount
	OvlOptions []string `toml:"ovl_mount_options"`

	// EnableFsverity enables fsverity for EROFS layers
	// Linux only
	EnableFsverity bool `toml:"enable_fsverity"`

	// If `SetImmutable` is enabled, IMMUTABLE_FL will be set on layer blobs.
	SetImmutable bool `toml:"set_immutable"`

	// DefaultSize is the default size of a writable layer in string
	DefaultSize string `toml:"default_size"`

	// DmverityMode controls dm-verity behavior: "auto" (use if available), "on" (require), "off" (disable)
	// Linux only
	DmverityMode string `toml:"dmverity_mode"`

	// LayerContentCaches lists directories of pre-converted, diffID-keyed erofs
	// layer blobs. A layer found in one of them is mounted from that directory,
	// in both sequential and parallel unpack modes. The directories are
	// searched in the order given and the first one holding the layer wins; one
	// that doesn't exist is treated as a miss. A layer found in none of them is
	// converted normally.
	//
	// The blobs are read but never written and are expected to outlive the
	// snapshots using them. Removing one while an image still refers to it
	// leaves that image unusable.
	LayerContentCaches []string `toml:"layer_content_caches"`
}

// snapshotterPlatforms returns the platforms this snapshotter advertises: the
// default platform, plus the "erofs" OS feature platform (see the EROFS image
// layer format specification, https://github.com/erofs/erofs-image-spec) so
// that clients pulling with --snapshotter erofs prefer the native EROFS image
// variant when one is available in a multi-platform index.
func snapshotterPlatforms() []ocispec.Platform {
	erofsPlatform := platforms.DefaultSpec()
	result := []ocispec.Platform{erofsPlatform}
	erofsPlatform.OSFeatures = []string{"erofs"}
	return append(result, erofsPlatform)
}

func init() {
	registry.Register(&plugin.Registration{
		Type:   plugins.SnapshotPlugin,
		ID:     "erofs",
		Config: &Config{},
		InitFn: func(ic *plugin.InitContext) (any, error) {
			ic.Meta.Platforms = append(ic.Meta.Platforms, snapshotterPlatforms()...)

			config, ok := ic.Config.(*Config)
			if !ok {
				return nil, errors.New("invalid erofs configuration")
			}

			var opts []erofs.Opt
			root := ic.Properties[plugins.PropertyRootDir]
			if len(config.RootPath) != 0 {
				root = config.RootPath
			}

			if len(config.OvlOptions) > 0 {
				opts = append(opts, erofs.WithOvlOptions(config.OvlOptions))
			}

			if config.EnableFsverity {
				opts = append(opts, erofs.WithFsverity())
			}

			if config.SetImmutable {
				opts = append(opts, erofs.WithImmutable())
			}

			if config.DefaultSize != "" {
				size, err := units.RAMInBytes(config.DefaultSize)
				if err != nil {
					return nil, fmt.Errorf("failed to parse default_size '%v': %w", config.DefaultSize, err)
				}
				opts = append(opts, erofs.WithDefaultSize(size))
			}

			if config.DmverityMode != "" {
				opts = append(opts, erofs.WithDmverityMode(config.DmverityMode))
			}

			if len(config.LayerContentCaches) > 0 {
				opts = append(opts, erofs.WithLayerContentCaches(config.LayerContentCaches...))
			}

			// Don't bother supporting overlay's slow_chown, only RemapIDs
			ic.Meta.Capabilities = append(ic.Meta.Capabilities, capaOnlyRemapIDs)
			if ok, err := supportsIDMappedMounts(); err == nil && ok {
				opts = append(opts, erofs.WithRemapIDs())
				ic.Meta.Capabilities = append(ic.Meta.Capabilities, capaRemapIDs)
			}

			ic.Meta.Exports[plugins.SnapshotterRootDir] = root
			ic.Meta.Capabilities = append(ic.Meta.Capabilities, snapshots.RebaseCap)
			return erofs.NewSnapshotter(root, opts...)
		},
	})
}
