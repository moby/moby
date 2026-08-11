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
	"iter"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"dario.cat/mergo"
	"github.com/pelletier/go-toml/v2"

	"github.com/containerd/containerd/v2/version"
	"github.com/containerd/errdefs"
	"github.com/containerd/log"
	"github.com/containerd/plugin"
)

// migrations hold the migration functions for every prior containerd config version
var migrations = []func(context.Context, *Config) error{
	nil,            // Version 0 is not defined, treated at version 1
	v1Migrate,      // Version 1 plugins renamed to URI for version 2
	nil,            // Version 2 has only plugin changes to version 3
	serviceMigrate, // Version 3 has server properties moved to plugins for version 4
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
	// Debug log settings
	Debug Debug `toml:"debug"`
	// DisabledPlugins are IDs of plugins to disable. Disabled plugins won't be
	// initialized and started.
	// DisabledPlugins must use a fully qualified plugin URI.
	DisabledPlugins []string `toml:"disabled_plugins"`
	// RequiredPlugins are IDs of required plugins. Containerd exits if any
	// required plugin doesn't exist or fails to be initialized or started.
	// RequiredPlugins must use a fully qualified plugin URI.
	RequiredPlugins []string `toml:"required_plugins"`
	// Plugins provides plugin specific configuration for the initialization of a plugin
	Plugins map[string]any `toml:"plugins"`
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

	// Deprecated fields must remain but should not be output when generating default or migrated configs.
	// These fields are automatically migrated to the corresponding server plugin
	// configuration blocks on startup (see serviceMigrate). In version 4 configs,
	// server settings are configured directly under [plugins."<plugin-id>"].

	// Deprecated: use server plugins io.containerd.server.v1.grpc and io.containerd.server.v1.grpc-tcp
	GRPC GRPCConfig `toml:"grpc,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.ttrpc.
	// In configs prior to version 4, an unset TTRPC address is derived from
	// the GRPC address (grpcAddress + ".ttrpc") and inherits GRPC's UID/GID.
	// In version 4, the TTRPC plugin uses its own defaults independently.
	TTRPC TTRPCConfig `toml:"ttrpc,omitempty"`
	// Metrics and monitoring settings
	// Deprecated: use server plugin io.containerd.server.v1.metrics
	Metrics MetricsConfig `toml:"metrics,omitempty"`
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
	plugins := make(map[string]any, len(c.Plugins))
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

// serviceMigrate moves server properties (GRPC, TTRPC, Debug, Metrics) from
// top-level config fields into their corresponding plugin configuration blocks
// for version 4.
//
// For configs prior to version 4, if the TTRPC address is not explicitly set
// it is derived from the GRPC address (grpcAddress + ".ttrpc") and inherits
// GRPC's UID/GID. In version 4, each server plugin is independently
// configured; the TTRPC plugin will use its own default address if its
// plugin block is omitted, regardless of the GRPC plugin's address.
func serviceMigrate(ctx context.Context, c *Config) error {
	if c.Plugins == nil {
		c.Plugins = make(map[string]any)
	}
	if c.Debug.Address != "" && c.Plugins["io.containerd.server.v1.debug"] == nil {
		c.Plugins["io.containerd.server.v1.debug"] = map[string]any{
			"address": c.Debug.Address,
			"uid":     c.Debug.UID,
			"gid":     c.Debug.GID,
		}
		c.Debug.Address = ""
		c.Debug.UID = 0
		c.Debug.GID = 0
	}
	// Capture legacy GRPC values up front so both grpc and grpc-tcp
	// migrations see the same values, and so migrations that don't key on
	// an address (uid/gid/max message sizes) are not dropped.
	grpcAddress := c.GRPC.Address
	grpcUID := c.GRPC.UID
	grpcGID := c.GRPC.GID
	grpcMaxRecv := c.GRPC.MaxRecvMsgSize
	grpcMaxSend := c.GRPC.MaxSendMsgSize
	grpcHasLegacy := grpcAddress != "" || grpcUID != 0 || grpcGID != 0 || grpcMaxRecv != 0 || grpcMaxSend != 0
	if grpcHasLegacy && c.Plugins["io.containerd.server.v1.grpc"] == nil {
		grpcConfig := map[string]any{}
		if grpcAddress != "" {
			grpcConfig["address"] = grpcAddress
			// Preserve legacy socket ownership semantics. In v3 configs, uid/gid
			// default to 0 and cannot be distinguished from an explicit 0.
			grpcConfig["uid"] = grpcUID
			grpcConfig["gid"] = grpcGID
		} else {
			if grpcUID != 0 {
				grpcConfig["uid"] = grpcUID
			}
			if grpcGID != 0 {
				grpcConfig["gid"] = grpcGID
			}
		}
		if grpcMaxRecv != 0 {
			grpcConfig["max_recv_message_size"] = grpcMaxRecv
		}
		if grpcMaxSend != 0 {
			grpcConfig["max_send_message_size"] = grpcMaxSend
		}
		c.Plugins["io.containerd.server.v1.grpc"] = grpcConfig
	}
	if c.GRPC.TCPAddress != "" && c.Plugins["io.containerd.server.v1.grpc-tcp"] == nil {
		grpcTCPConfig := map[string]any{
			"address": c.GRPC.TCPAddress,
		}
		if c.GRPC.TCPTLSCA != "" {
			grpcTCPConfig["tls_ca"] = c.GRPC.TCPTLSCA
		}
		if c.GRPC.TCPTLSCert != "" {
			grpcTCPConfig["tls_cert"] = c.GRPC.TCPTLSCert
		}
		if c.GRPC.TCPTLSKey != "" {
			grpcTCPConfig["tls_key"] = c.GRPC.TCPTLSKey
		}
		if c.GRPC.TCPTLSCName != "" {
			grpcTCPConfig["tls_common_name"] = c.GRPC.TCPTLSCName
		}
		if grpcMaxRecv != 0 {
			grpcTCPConfig["max_recv_message_size"] = grpcMaxRecv
		}
		if grpcMaxSend != 0 {
			grpcTCPConfig["max_send_message_size"] = grpcMaxSend
		}
		c.Plugins["io.containerd.server.v1.grpc-tcp"] = grpcTCPConfig
	}
	if grpcHasLegacy || c.GRPC.TCPAddress != "" {
		c.GRPC = GRPCConfig{}
	}
	if c.Plugins["io.containerd.server.v1.ttrpc"] == nil {
		ttrpcAddress := c.TTRPC.Address
		ttrpcUID := c.TTRPC.UID
		ttrpcGID := c.TTRPC.GID
		if ttrpcAddress == "" && grpcAddress != "" {
			ttrpcAddress = grpcAddress + ".ttrpc"
			if ttrpcUID == 0 {
				ttrpcUID = grpcUID
			}
			if ttrpcGID == 0 {
				ttrpcGID = grpcGID
			}
		}
		if ttrpcAddress != "" || ttrpcUID != 0 || ttrpcGID != 0 {
			c.Plugins["io.containerd.server.v1.ttrpc"] = map[string]any{
				"address": ttrpcAddress,
				"uid":     ttrpcUID,
				"gid":     ttrpcGID,
			}
			c.TTRPC.Address = ""
			c.TTRPC.UID = 0
			c.TTRPC.GID = 0
		}
	}
	if c.Metrics.GRPCHistogram && c.Plugins["io.containerd.metrics.v1.grpc-prometheus"] == nil {
		c.Plugins["io.containerd.metrics.v1.grpc-prometheus"] = map[string]any{
			"grpc_histogram": c.Metrics.GRPCHistogram,
		}
		c.Metrics.GRPCHistogram = false
	}
	if c.Metrics.Address != "" && c.Plugins["io.containerd.server.v1.metrics"] == nil {
		c.Plugins["io.containerd.server.v1.metrics"] = map[string]any{
			"address": c.Metrics.Address,
		}
		c.Metrics.Address = ""
	}
	return nil
}

// GRPCConfig provides GRPC configuration for the socket
type GRPCConfig struct {
	// Deprecated: use server plugin io.containerd.server.v1.grpc
	Address string `toml:"address,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc-tcp
	TCPAddress string `toml:"tcp_address,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc-tcp
	TCPTLSCA string `toml:"tcp_tls_ca,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc-tcp
	TCPTLSCert string `toml:"tcp_tls_cert,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc-tcp
	TCPTLSKey string `toml:"tcp_tls_key,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc
	UID int `toml:"uid,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc
	GID int `toml:"gid,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc
	MaxRecvMsgSize int `toml:"max_recv_message_size,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc
	MaxSendMsgSize int `toml:"max_send_message_size,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.grpc-tcp
	TCPTLSCName string `toml:"tcp_tls_common_name,omitempty"`
}

// TTRPCConfig provides TTRPC configuration for the socket
type TTRPCConfig struct {
	Address string `toml:"address,omitempty"`
	UID     int    `toml:"uid,omitempty"`
	GID     int    `toml:"gid,omitempty"`
}

// Debug provides debug configuration
type Debug struct {
	Level string `toml:"level"`
	// Format represents the logging format. Supported values are 'text' and 'json'.
	Format     string `toml:"format"`
	LogTraceID bool   `toml:"log_trace_id"`

	// Deprecated: use server plugin io.containerd.server.v1.debug
	Address string `toml:"address,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.debug
	UID int `toml:"uid,omitempty"`
	// Deprecated: use server plugin io.containerd.server.v1.debug
	GID int `toml:"gid,omitempty"`
}

// MetricsConfig provides metrics configuration
type MetricsConfig struct {
	// Deprecated: use server plugin io.containerd.server.v1.metrics
	Address string `toml:"address,omitempty"`
	// Deprecated: use metrics plugin io.containerd.metrics.v1.grpc-prometheus
	GRPCHistogram bool `toml:"grpc_histogram,omitempty"`
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
func (c *Config) Decode(ctx context.Context, id string, config any) (any, error) {
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
	return LoadConfigWithPlugins(ctx, path, nil, out)
}

// PluginFunc returns an iterator to the plugin registrations
type PluginFunc func() iter.Seq[plugin.Registration]

// LoadConfigWithPlugins loads the containerd server config from the provided path
// and using the migration functions from the provided plugins.
func LoadConfigWithPlugins(ctx context.Context, path string, plugins PluginFunc, out *Config) error {
	if out == nil {
		return fmt.Errorf("argument out must not be nil: %w", errdefs.ErrInvalidArgument)
	}

	var (
		loaded            = map[string]bool{}
		pending           = []string{path}
		rootConfigVersion = 0
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

		// Check to make sure drop-in configs does not have a higher version than the root config version
		if rootConfigVersion == 0 {
			rootConfigVersion = config.Version
		}
		if config.Version > rootConfigVersion {
			return fmt.Errorf("drop-in config version %d higher than root config version %d", config.Version, rootConfigVersion)
		}

		if config.Version < out.Version {
			var (
				currentVersion = config.Version
				t1             = time.Now()
			)
			for v := currentVersion; v < out.Version; v++ {
				if err := config.MigrateConfigTo(ctx, v+1); err != nil {
					return err
				}
				if plugins != nil {
					// Run migration for each configuration version
					// Run each plugin migration for each version to ensure that migration logic is simple and
					// focused on upgrading from one version at a time.
					for p := range plugins() {
						if p.ConfigMigration != nil {
							if err := p.ConfigMigration(ctx, v, config.Plugins); err != nil {
								return err
							}
						}
					}
				}
			}
			log.G(ctx).WithField("t", time.Since(t1)).Warnf("Configuration migrated from version %d, use `containerd config migrate` to avoid migration", currentVersion)
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
	maps.Copy(to.StreamProcessors, from.StreamProcessors)

	maps.Copy(to.ProxyPlugins, from.ProxyPlugins)

	maps.Copy(to.Timeouts, from.Timeouts)

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
