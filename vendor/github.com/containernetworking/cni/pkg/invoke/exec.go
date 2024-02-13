// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package invoke

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/create"
	"github.com/containernetworking/cni/pkg/version"
)

// Exec is an interface encapsulates all operations that deal with finding
// and executing a CNI plugin. Tests may provide a fake implementation
// to avoid writing fake plugins to temporary directories during the test.
type Exec interface {
	ExecPlugin(ctx context.Context, pluginPath string, stdinData []byte, environ []string) ([]byte, error)
	FindInPath(plugin string, paths []string) (string, error)
	Decode(jsonBytes []byte) (version.PluginInfo, error)
}

// Plugin must return result in same version as specified in netconf; but
// for backwards compatibility reasons if the result version is empty use
// config version (rather than technically correct 0.1.0).
// https://github.com/containernetworking/cni/issues/895
func fixupResultVersion(netconf, result []byte) (string, []byte, error) {
	versionDecoder := &version.ConfigDecoder{}
	confVersion, err := versionDecoder.Decode(netconf)
	if err != nil {
		return "", nil, err
	}

	var rawResult map[string]interface{}
	if err := json.Unmarshal(result, &rawResult); err != nil {
		return "", nil, fmt.Errorf("failed to unmarshal raw result: %w", err)
	}

	// plugin output of "null" is successfully unmarshalled, but results in a nil
	// map which causes a panic when the confVersion is assigned below.
	if rawResult == nil {
		rawResult = make(map[string]interface{})
	}

	// Manually decode Result version; we need to know whether its cniVersion
	// is empty, while built-in decoders (correctly) substitute 0.1.0 for an
	// empty version per the CNI spec.
	if resultVerRaw, ok := rawResult["cniVersion"]; ok {
		resultVer, ok := resultVerRaw.(string)
		if ok && resultVer != "" {
			return resultVer, result, nil
		}
	}

	// If the cniVersion is not present or empty, assume the result is
	// the same CNI spec version as the config
	rawResult["cniVersion"] = confVersion
	newBytes, err := json.Marshal(rawResult)
	if err != nil {
		return "", nil, fmt.Errorf("failed to remarshal fixed result: %w", err)
	}

	return confVersion, newBytes, nil
}

// For example, a testcase could pass an instance of the following fakeExec
// object to ExecPluginWithResult() to verify the incoming stdin and environment
// and provide a tailored response:
//
//import (
//	"encoding/json"
//	"path"
//	"strings"
//)
//
//type fakeExec struct {
//	version.PluginDecoder
//}
//
//func (f *fakeExec) ExecPlugin(pluginPath string, stdinData []byte, environ []string) ([]byte, error) {
//	net := &types.NetConf{}
//	err := json.Unmarshal(stdinData, net)
//	if err != nil {
//		return nil, fmt.Errorf("failed to unmarshal configuration: %v", err)
//	}
//	pluginName := path.Base(pluginPath)
//	if pluginName != net.Type {
//		return nil, fmt.Errorf("plugin name %q did not match config type %q", pluginName, net.Type)
//	}
//	for _, e := range environ {
//		// Check environment for forced failure request
//		parts := strings.Split(e, "=")
//		if len(parts) > 0 && parts[0] == "FAIL" {
//			return nil, fmt.Errorf("failed to execute plugin %s", pluginName)
//		}
//	}
//	return []byte("{\"CNIVersion\":\"0.4.0\"}"), nil
//}
//
//func (f *fakeExec) FindInPath(plugin string, paths []string) (string, error) {
//	if len(paths) > 0 {
//		return path.Join(paths[0], plugin), nil
//	}
//	return "", fmt.Errorf("failed to find plugin %s in paths %v", plugin, paths)
//}

func ExecPluginWithResult(ctx context.Context, pluginPath string, netconf []byte, args CNIArgs, exec Exec) (types.Result, error) {
	if exec == nil {
		exec = defaultExec
	}

	stdoutBytes, err := exec.ExecPlugin(ctx, pluginPath, netconf, args.AsEnv())
	if err != nil {
		return nil, err
	}

	resultVersion, fixedBytes, err := fixupResultVersion(netconf, stdoutBytes)
	if err != nil {
		return nil, err
	}

	return create.Create(resultVersion, fixedBytes)
}

func ExecPluginWithoutResult(ctx context.Context, pluginPath string, netconf []byte, args CNIArgs, exec Exec) error {
	if exec == nil {
		exec = defaultExec
	}
	_, err := exec.ExecPlugin(ctx, pluginPath, netconf, args.AsEnv())
	return err
}

// GetVersionInfo returns the version information available about the plugin.
// For recent-enough plugins, it uses the information returned by the VERSION
// command.  For older plugins which do not recognize that command, it reports
// version 0.1.0
func GetVersionInfo(ctx context.Context, pluginPath string, exec Exec) (version.PluginInfo, error) {
	if exec == nil {
		exec = defaultExec
	}
	args := &Args{
		Command: "VERSION",

		// set fake values required by plugins built against an older version of skel
		NetNS:  "dummy",
		IfName: "dummy",
		Path:   "dummy",
	}
	stdin := []byte(fmt.Sprintf(`{"cniVersion":%q}`, version.Current()))
	stdoutBytes, err := exec.ExecPlugin(ctx, pluginPath, stdin, args.AsEnv())
	if err != nil {
		if err.Error() == "unknown CNI_COMMAND: VERSION" {
			return version.PluginSupports("0.1.0"), nil
		}
		return nil, err
	}

	return exec.Decode(stdoutBytes)
}

// DefaultExec is an object that implements the Exec interface which looks
// for and executes plugins from disk.
type DefaultExec struct {
	*RawExec
	version.PluginDecoder
}

// DefaultExec implements the Exec interface
var _ Exec = &DefaultExec{}

var defaultExec = &DefaultExec{
	RawExec: &RawExec{Stderr: os.Stderr},
}
