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

package api

import (
	"fmt"
	"strings"
)

const (
	// DefaultSocketPath is the default socket path for external plugins.
	DefaultSocketPath = "/var/run/nri/nri.sock"
	// PluginSocketEnvVar is used to inform plugins about pre-connected sockets.
	PluginSocketEnvVar = "NRI_PLUGIN_SOCKET"
	// PluginNameEnvVar is used to inform NRI-launched plugins about their name.
	PluginNameEnvVar = "NRI_PLUGIN_NAME"
	// PluginIdxEnvVar is used to inform NRI-launched plugins about their ID.
	PluginIdxEnvVar = "NRI_PLUGIN_IDX"
)

// ParsePluginName parses the (file)name of a plugin into an index and a base.
func ParsePluginName(name string) (string, string, error) {
	split := strings.SplitN(name, "-", 2)
	if len(split) < 2 {
		return "", "", fmt.Errorf("invalid plugin name %q, idx-pluginname expected", name)
	}

	if err := CheckPluginIndex(split[0]); err != nil {
		return "", "", err
	}

	return split[0], split[1], nil
}

// CheckPluginIndex checks the validity of a plugin index.
func CheckPluginIndex(idx string) error {
	if len(idx) != 2 {
		return fmt.Errorf("invalid plugin index %q, must be 2 digits", idx)
	}
	if !('0' <= idx[0] && idx[0] <= '9') || !('0' <= idx[1] && idx[1] <= '9') {
		return fmt.Errorf("invalid plugin index %q (not [0-9][0-9])", idx)
	}
	return nil
}
