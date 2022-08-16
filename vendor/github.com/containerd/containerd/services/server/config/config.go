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

package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/imdario/mergo"
	"github.com/pelletier/go-toml"
	"github.com/sirupsen/logrus"

	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/plugin"
)

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
	// PluginDir is the directory for dynamic plugins to be stored
	PluginDir string `toml:"plugin_dir"`
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
	DisabledPlugins []string `toml:"disabled_plugins"`
	// RequiredPlugins are IDs of required plugins. Containerd exits if any
	// required plugin doesn't exist or fails to be initialized or started.
	RequiredPlugins []string `toml:"required_plugins"`
	// Plugins provides plugin specific configuration for the initialization of a plugin
	Plugins map[string]toml.Tree `toml:"plugins"`
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

// GetVersion returns the config file's version
func (c *Config) GetVersion() int {
	if c.Version == 0 {
		return 1
	}
	return c.Version
}

// ValidateV2 validates the config for a v2 file
func (c *Config) ValidateV2() error {
	version := c.GetVersion()
	if version < 2 {
		logrus.Warnf("containerd config version `%d` has been deprecated and will be removed in containerd v2.0, please switch to version `2`, "+
			"see https://github.com/containerd/containerd/blob/main/docs/PLUGINS.md#version-header", version)
		return nil
	}
	for _, p := range c.DisabledPlugins {
		if len(strings.Split(p, ".")) < 4 {
			return fmt.Errorf("invalid disabled plugin URI %q expect io.containerd.x.vx", p)
		}
	}
	for _, p := range c.RequiredPlugins {
		if len(strings.Split(p, ".")) < 4 {
			return fmt.Errorf("invalid required plugin URI %q expect io.containerd.x.vx", p)
		}
	}
	for p := range c.Plugins {
		if len(strings.Split(p, ".")) < 4 {
			return fmt.Errorf("invalid plugin key URI %q expect io.containerd.x.vx", p)
		}
	}
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
	// Format represents the logging format
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
	Type    string `toml:"type"`
	Address string `toml:"address"`
}

// BoltConfig defines the configuration values for the bolt plugin, which is
// loaded here, rather than back registered in the metadata package.
type BoltConfig struct {
	// ContentSharingPolicy sets the sharing policy for content between
	// namespaces.
	//
	// The default mode "shared" will make blobs available in all
	// namespaces once it is pulled into any namespace. The blob will be pulled
	// into the namespace if a writer is opened with the "Expected" digest that
	// is already present in the backend.
	//
	// The alternative mode, "isolated" requires that clients prove they have
	// access to the content by providing all of the content to the ingest
	// before the blob is added to the namespace.
	//
	// Both modes share backing data, while "shared" will reduce total
	// bandwidth across namespaces, at the cost of allowing access to any blob
	// just by knowing its digest.
	ContentSharingPolicy string `toml:"content_sharing_policy"`
}

const (
	// SharingPolicyShared represents the "shared" sharing policy
	SharingPolicyShared = "shared"
	// SharingPolicyIsolated represents the "isolated" sharing policy
	SharingPolicyIsolated = "isolated"
)

// Validate validates if BoltConfig is valid
func (bc *BoltConfig) Validate() error {
	switch bc.ContentSharingPolicy {
	case SharingPolicyShared, SharingPolicyIsolated:
		return nil
	default:
		return fmt.Errorf("unknown policy: %s: %w", bc.ContentSharingPolicy, errdefs.ErrInvalidArgument)
	}
}

// Decode unmarshals a plugin specific configuration by plugin id
func (c *Config) Decode(p *plugin.Registration) (interface{}, error) {
	id := p.URI()
	if c.GetVersion() == 1 {
		id = p.ID
	}
	data, ok := c.Plugins[id]
	if !ok {
		return p.Config, nil
	}
	if err := data.Unmarshal(p.Config); err != nil {
		return nil, err
	}
	return p.Config, nil
}

// LoadConfig loads the containerd server config from the provided path
func LoadConfig(path string, out *Config) error {
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

		config, err := loadConfigFile(path)
		if err != nil {
			return err
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

	// Fix up the list of config files loaded
	out.Imports = []string{}
	for path := range loaded {
		out.Imports = append(out.Imports, path)
	}

	err := out.ValidateV2()
	if err != nil {
		return fmt.Errorf("failed to load TOML from %s: %w", path, err)
	}
	return nil
}

// loadConfigFile decodes a TOML file at the given path
func loadConfigFile(path string) (*Config, error) {
	config := &Config{}

	file, err := toml.LoadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load TOML: %s: %w", path, err)
	}

	if err := file.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal TOML: %w", err)
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
		if strings.Contains(path, "*") {
			matches, err := filepath.Glob(path)
			if err != nil {
				return nil, err
			}

			out = append(out, matches...)
		} else {
			path = filepath.Clean(path)
			if !filepath.IsAbs(path) {
				path = filepath.Join(filepath.Dir(parent), path)
			}

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
// Maps merged by keys, but values are replaced entirely.
func mergeConfig(to, from *Config) error {
	err := mergo.Merge(to, from, mergo.WithOverride, mergo.WithAppendSlice)
	if err != nil {
		return err
	}

	// Replace entire sections instead of merging map's values.
	for k, v := range from.Plugins {
		to.Plugins[k] = v
	}

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

// V1DisabledFilter matches based on ID
func V1DisabledFilter(list []string) plugin.DisableFilter {
	set := make(map[string]struct{}, len(list))
	for _, l := range list {
		set[l] = struct{}{}
	}
	return func(r *plugin.Registration) bool {
		_, ok := set[r.ID]
		return ok
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
