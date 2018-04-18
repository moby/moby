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

package server

import (
	"github.com/BurntSushi/toml"
	"github.com/containerd/containerd/errdefs"
	"github.com/pkg/errors"
)

// Config provides containerd configuration data for the server
type Config struct {
	// Root is the path to a directory where containerd will store persistent data
	Root string `toml:"root"`
	// State is the path to a directory where containerd will store transient data
	State string `toml:"state"`
	// GRPC configuration settings
	GRPC GRPCConfig `toml:"grpc"`
	// Debug and profiling settings
	Debug Debug `toml:"debug"`
	// Metrics and monitoring settings
	Metrics MetricsConfig `toml:"metrics"`
	// DisabledPlugins are IDs of plugins to disable. Disabled plugins won't be
	// initialized and started.
	DisabledPlugins []string `toml:"disabled_plugins"`
	// Plugins provides plugin specific configuration for the initialization of a plugin
	Plugins map[string]toml.Primitive `toml:"plugins"`
	// OOMScore adjust the containerd's oom score
	OOMScore int `toml:"oom_score"`
	// Cgroup specifies cgroup information for the containerd daemon process
	Cgroup CgroupConfig `toml:"cgroup"`

	md toml.MetaData
}

// GRPCConfig provides GRPC configuration for the socket
type GRPCConfig struct {
	Address        string `toml:"address"`
	UID            int    `toml:"uid"`
	GID            int    `toml:"gid"`
	MaxRecvMsgSize int    `toml:"max_recv_message_size"`
	MaxSendMsgSize int    `toml:"max_send_message_size"`
}

// Debug provides debug configuration
type Debug struct {
	Address string `toml:"address"`
	UID     int    `toml:"uid"`
	GID     int    `toml:"gid"`
	Level   string `toml:"level"`
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

// Decode unmarshals a plugin specific configuration by plugin id
func (c *Config) Decode(id string, v interface{}) (interface{}, error) {
	data, ok := c.Plugins[id]
	if !ok {
		return v, nil
	}
	if err := c.md.PrimitiveDecode(data, v); err != nil {
		return nil, err
	}
	return v, nil
}

// LoadConfig loads the containerd server config from the provided path
func LoadConfig(path string, v *Config) error {
	if v == nil {
		return errors.Wrapf(errdefs.ErrInvalidArgument, "argument v must not be nil")
	}
	md, err := toml.DecodeFile(path, v)
	if err != nil {
		return err
	}
	v.md = md
	return nil
}
