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

// config is the global configuration for containerd
//
// Version History
// 1: Deprecated and removed in containerd 2.0
// 2: Uses fully qualified plugin names
// 3: Added support for migration and warning on unknown fields
package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"dario.cat/mergo"
	"github.com/pelletier/go-toml/v2"

	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
)

// migrations hold the migration functions for every prior containerd config version
var migrations = []func(context.Context, *Config) error{
	nil,       // Version 0 is not defined, treated at version 1
	v1Migrate, // Version 1 plugins renamed to URI for version 2
	nil,       // Version 2 has only plugin changes to version 3
}

// NOTE: Any new map fields added also need to be handled in mergeConfig.

// Config provides containerd configuration data for the server
type Config struct {
	// Version of the config file
	Version int `toml:"version"`
	// Root is the path to a directory where containerd will store persistent data
	Root string `toml:"root"`
	// State is the path to a directory where containerd will store transient data
	State string `toml:"state"`
	// TempDir is the path to a directory where to place containerd temporary files
	TempDir string `toml:"temp"`
	// GRPC configuration settings
	GRPC GRPCConfig `toml:"grpc"`
	// TTRPC configuration settings
	TTRPC TTRPCConfig `toml:"ttrpc"`
	// Debug and profiling settings
	Debug Debug `toml:"debug"`
	// Metrics and monitoring settings
	Metrics MetricsConfig `toml:"metrics"`
	// DisabledPlugins are IDs of plugins to disable. Disabled plugins won't be
	// initialized and started.
	// DisabledPlugins must use a fully qualified plugin URI.
	DisabledPlugins []string `toml:"disabled_plugins"`
	// RequiredPlugins are IDs of required plugins. Containerd exits if any
	// required plugin doesn't exist or fails to be initialized or started.
	// RequiredPlugins must use a fully qualified plugin URI.
	RequiredPlugins []string `toml:"required_plugins"`
	// Plugins provides plugin specific configuration for the initialization of a plugin
	Plugins map[string]interface{} `toml:"plugins"`
	// OOMScore adjust the containerd's oom score
	OOMScore int `toml:"oom_score"`
	// Cgroup specifies cgroup information for the containerd daemon process
	Cgroup CgroupConfig `toml:"cgroup"`
	// ProxyPlugins configures plugins which are communicated to over GRPC
	ProxyPlugins map[string]ProxyPlugin `toml:"proxy_plugins"`
	// Timeouts specified as a duration
	Timeouts map[string]string `toml:"timeouts"`
	// Imports are additional file path list to config files that can overwrite main config file fields
	Imports []string `toml:"imports"`
	// StreamProcessors configuration
	StreamProcessors map[string]StreamProcessor `toml:"stream_processors"`
}

// StreamProcessor provides configuration for diff content processors
type StreamProcessor struct {
	// Accepts specific media-types
	Accepts []string `toml:"accepts"`
	// Returns the media-type
	Returns string `toml:"returns"`
	// Path or name of the binary
	Path string `toml:"path"`
	// Args to the binary
	Args []string `toml:"args"`
	// Environment variables for the binary
	Env []string `toml:"env"`
}

// ValidateVersion validates the config for a v2 file
func (c *Config) ValidateVersion() error {
	if c.Version > version.ConfigVersion {
		return fmt.Errorf("expected containerd config version equal to or less than `%d`, got `%d`", version.ConfigVersion, c.Version)
	}

	for _, p := range c.DisabledPlugins {
		if !strings.ContainsAny(p, ".") {
			return fmt.Errorf("invalid disabled plugin URI %q expect io.containerd.x.vx", p)
		}
	}
	for _, p := range c.RequiredPlugins {
		if !strings.ContainsAny(p, ".") {
			return fmt.Errorf("invalid required plugin URI %q expect io.containerd.x.vx", p)
		}
	}

	return nil
}

// MigrateConfig will convert the config to the latest version before using
func (c *Config) MigrateConfig(ctx context.Context) error {
	return c.MigrateConfigTo(ctx, version.ConfigVersion)
}

// MigrateConfigTo will convert the config to the target version before using
func (c *Config) MigrateConfigTo(ctx context.Context, targetVersion int) error {
	for c.Version < targetVersion {
		if m := migrations[c.Version]; m != nil {
			if err := m(ctx, c); err != nil {
				return err
			}
		}
		c.Version++
	}
	return nil
}

func v1MigratePluginName(ctx context.Context, plugin string) string {
	// corePlugins is the list of used plugins before v1 was deprecated
	corePlugins := map[string]string{
		"cri":       "io.containerd.grpc.v1.cri",
		"cgroups":   "io.containerd.monitor.v1.cgroups",
		"linux":     "io.containerd.runtime.v1.linux",
		"scheduler": "io.containerd.gc.v1.scheduler",
		"bolt":      "io.containerd.metadata.v1.bolt",
		"task":      "io.containerd.runtime.v2.task",
		"opt":       "io.containerd.internal.v1.opt",
		"restart":   "io.containerd.internal.v1.restart",
		"tracing":   "io.containerd.internal.v1.tracing",
		"otlp":      "io.containerd.tracing.processor.v1.otlp",
		"aufs":      "io.containerd.snapshotter.v1.aufs",
		"btrfs":     "io.containerd.snapshotter.v1.btrfs",
		"devmapper": "io.containerd.snapshotter.v1.devmapper",
		"native":    "io.containerd.snapshotter.v1.native",
		"overlayfs": "io.containerd.snapshotter.v1.overlayfs",
		"zfs":       "io.containerd.snapshotter.v1.zfs",
	}
	if !strings.ContainsAny(plugin, ".") {
		var ambiguous string
		if full, ok := corePlugins[plugin]; ok {
			plugin = full
		} else if strings.HasSuffix(plugin, "-service") {
			plugin = "io.containerd.service.v1." + plugin
		} else if plugin == "windows" || plugin == "windows-lcow" {
			// runtime, differ, and snapshotter plugins do not have configs for v1
			ambiguous = plugin
			plugin = "io.containerd.snapshotter.v1." + plugin
		} else {
			ambiguous = plugin
			plugin = "io.containerd.grpc.v1." + plugin
		}
		if ambiguous != "" {
			log.G(ctx).Warnf("Ambiguous %s plugin in v1 config, treating as %s", ambiguous, plugin)
		}
	}
	return plugin
}

func v1Migrate(ctx context.Context, c *Config) error {
	plugins := make(map[string]interface{}, len(c.Plugins))
	for plugin, value := range c.Plugins {
		plugins[v1MigratePluginName(ctx, plugin)] = value
	}
	c.Plugins = plugins
	for i, plugin := range c.DisabledPlugins {
		c.DisabledPlugins[i] = v1MigratePluginName(ctx, plugin)
	}
	for i, plugin := range c.RequiredPlugins {
		c.RequiredPlugins[i] = v1MigratePluginName(ctx, plugin)
	}
	// No change in c.ProxyPlugins
	return nil
}

// GRPCConfig provides GRPC configuration for the socket
type GRPCConfig struct {
	Address        string `toml:"address"`
	TCPAddress     string `toml:"tcp_address"`
	TCPTLSCA       string `toml:"tcp_tls_ca"`
	TCPTLSCert     string `toml:"tcp_tls_cert"`
	TCPTLSKey      string `toml:"tcp_tls_key"`
	UID            int    `toml:"uid"`
	GID            int    `toml:"gid"`
	MaxRecvMsgSize int    `toml:"max_recv_message_size"`
	MaxSendMsgSize int    `toml:"max_send_message_size"`
}

// TTRPCConfig provides TTRPC configuration for the socket
type TTRPCConfig struct {
	Address string `toml:"address"`
	UID     int    `toml:"uid"`
	GID     int    `toml:"gid"`
}

// Debug provides debug configuration
type Debug struct {
	Address string `toml:"address"`
	UID     int    `toml:"uid"`
	GID     int    `toml:"gid"`
	Level   string `toml:"level"`
	// Format represents the logging format. Supported values are 'text' and 'json'.
	Format string `toml:"format"`
}

// MetricsConfig provides metrics configuration
type MetricsConfig struct {
	Address       string `toml:"address"`
	GRPCHistogram bool   `toml:"grpc_histogram"`
}

// CgroupConfig provides cgroup configuration
type CgroupConfig struct {
	Path string `toml:"path"`
}

// ProxyPlugin provides a proxy plugin configuration
type ProxyPlugin struct {
	Type         string            `toml:"type"`
	Address      string            `toml:"address"`
	Platform     string            `toml:"platform"`
	Exports      map[string]string `toml:"exports"`
	Capabilities []string          `toml:"capabilities"`
}

// Decode unmarshals a plugin specific configuration by plugin id
func (c *Config) Decode(ctx context.Context, id string, config interface{}) (interface{}, error) {
	data, ok := c.Plugins[id]
	if !ok {
		return config, nil
	}

	b, err := toml.Marshal(data)
	if err != nil {
		return nil, err
	}

	if err := toml.NewDecoder(bytes.NewReader(b)).DisallowUnknownFields().Decode(config); err != nil {
		var serr *toml.StrictMissingError
		if errors.As(err, &serr) {
			for _, derr := range serr.Errors {
				log.G(ctx).WithFields(log.Fields{
					"plugin": id,
					"key":    strings.Join(derr.Key(), " "),
				}).WithError(err).Warn("Ignoring unknown key in TOML for plugin")
			}
			err = toml.Unmarshal(b, config)
		}
		if err != nil {
			return nil, err
		}

	}

	return config, nil
}

// LoadConfig loads the containerd server config from the provided path
func LoadConfig(ctx context.Context, path string, out *Config) error {
	if out == nil {
		return fmt.Errorf("argument out must not be nil: %w", errdefs.ErrInvalidArgument)
	}

	var (
		loaded  = map[string]bool{}
		pending = []string{path}
	)

	for len(pending) > 0 {
		path, pending = pending[0], pending[1:]

		// Check if a file at the given path already loaded to prevent circular imports
		if _, ok := loaded[path]; ok {
			continue
		}

		config, err := loadConfigFile(ctx, path)
		if err != nil {
			return err
		}

		switch config.Version {
		case 0, 1:
			if err := config.MigrateConfigTo(ctx, out.Version); err != nil {
				return err
			}
		default:
			// NOP
		}

		if err := mergeConfig(out, config); err != nil {
			return err
		}

		imports, err := resolveImports(path, config.Imports)
		if err != nil {
			return err
		}

		loaded[path] = true
		pending = append(pending, imports...)
	}

	err := out.ValidateVersion()
	if err != nil {
		return fmt.Errorf("failed to load TOML from %s: %w", path, err)
	}
	return nil
}

// loadConfigFile decodes a TOML file at the given path
func loadConfigFile(ctx context.Context, path string) (*Config, error) {
	config := &Config{}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	if err := toml.NewDecoder(f).DisallowUnknownFields().Decode(config); err != nil {
		var serr *toml.StrictMissingError
		if errors.As(err, &serr) {
			for _, derr := range serr.Errors {
				row, col := derr.Position()
				log.G(ctx).WithFields(log.Fields{
					"file":   path,
					"row":    row,
					"column": col,
					"key":    strings.Join(derr.Key(), " "),
				}).WithError(err).Warn("Ignoring unknown key in TOML")
			}

			// Try decoding again with unknown fields
			config = &Config{}
			if _, seekerr := f.Seek(0, io.SeekStart); seekerr != nil {
				return nil, fmt.Errorf("unable to seek file to start %w: failed to unmarshal TOML with unknown fields: %w", seekerr, err)
			}
			err = toml.NewDecoder(f).Decode(config)
		}
		if err != nil {
			var derr *toml.DecodeError
			if errors.As(err, &derr) {
				row, column := derr.Position()
				log.G(ctx).WithFields(log.Fields{
					"file":   path,
					"row":    row,
					"column": column,
				}).WithError(err).Error("Failure unmarshaling TOML")
				return nil, fmt.Errorf("failed to unmarshal TOML at row %d column %d: %w", row, column, err)
			}
			return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
		}

	}

	return config, nil
}

// resolveImports resolves import strings list to absolute paths list:
// - If path contains *, glob pattern matching applied
// - Non abs path is relative to parent config file directory
// - Abs paths returned as is
func resolveImports(parent string, imports []string) ([]string, error) {
	var out []string

	for _, path := range imports {
		path = filepath.Clean(path)
		if !filepath.IsAbs(path) {
			path = filepath.Join(filepath.Dir(parent), path)
		}

		if strings.Contains(path, "*") {
			matches, err := filepath.Glob(path)
			if err != nil {
				return nil, err
			}

			out = append(out, matches...)
		} else {
			out = append(out, path)
		}
	}

	return out, nil
}

// mergeConfig merges Config structs with the following rules:
// 'to'         'from'      'result'
// ""           "value"     "value"
// "value"      ""          "value"
// 1            0           1
// 0            1           1
// []{"1"}      []{"2"}     []{"1","2"}
// []{"1"}      []{}        []{"1"}
// []{"1", "2"} []{"1"}     []{"1","2"}
// []{}         []{"2"}     []{"2"}
// Maps merged by keys, but values are replaced entirely.
func mergeConfig(to, from *Config) error {
	err := mergo.Merge(to, from, mergo.WithOverride, mergo.WithTransformers(sliceTransformer{}))
	if err != nil {
		return err
	}

	// Replace entire sections instead of merging map's values.
	for k, v := range from.StreamProcessors {
		to.StreamProcessors[k] = v
	}

	for k, v := range from.ProxyPlugins {
		to.ProxyPlugins[k] = v
	}

	for k, v := range from.Timeouts {
		to.Timeouts[k] = v
	}

	return nil
}

type sliceTransformer struct{}

func (sliceTransformer) Transformer(t reflect.Type) func(dst, src reflect.Value) error {
	if t.Kind() != reflect.Slice {
		return nil
	}
	return func(dst, src reflect.Value) error {
		if !dst.CanSet() {
			return nil
		}
		if src.Type() != dst.Type() {
			return fmt.Errorf("cannot append two slice with different type (%s, %s)", src.Type(), dst.Type())
		}
		for i := 0; i < src.Len(); i++ {
			found := false
			for j := 0; j < dst.Len(); j++ {
				srcv := src.Index(i)
				dstv := dst.Index(j)
				if !srcv.CanInterface() || !dstv.CanInterface() {
					if srcv.Equal(dstv) {
						found = true
						break
					}
				} else if reflect.DeepEqual(srcv.Interface(), dstv.Interface()) {
					found = true
					break
				}
			}
			if !found {
				dst.Set(reflect.Append(dst, src.Index(i)))
			}
		}

		return nil
	}
}

// V2DisabledFilter matches based on URI
func V2DisabledFilter(list []string) plugin.DisableFilter {
	set := make(map[string]struct{}, len(list))
	for _, l := range list {
		set[l] = struct{}{}
	}
	return func(r *plugin.Registration) bool {
		_, ok := set[r.URI()]
		return ok
	}
}
