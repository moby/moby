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

package libcni

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/version"
)

type NotFoundError struct {
	Dir  string
	Name string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf(`no net configuration with name "%s" in %s`, e.Name, e.Dir)
}

type NoConfigsFoundError struct {
	Dir string
}

func (e NoConfigsFoundError) Error() string {
	return fmt.Sprintf(`no net configurations found in %s`, e.Dir)
}

// This will not validate that the plugins actually belong to the netconfig by ensuring
// that they are loaded from a directory named after the networkName, relative to the network config.
//
// Since here we are just accepting raw bytes, the caller is responsible for ensuring that the plugin
// config provided here actually "belongs" to the networkconfig in question.
func NetworkPluginConfFromBytes(pluginConfBytes []byte) (*PluginConfig, error) {
	// TODO why are we creating a struct that holds both the byte representation and the deserialized
	// representation, and returning that, instead of just returning the deserialized representation?
	conf := &PluginConfig{Bytes: pluginConfBytes, Network: &types.PluginConf{}}
	if err := json.Unmarshal(pluginConfBytes, conf.Network); err != nil {
		return nil, fmt.Errorf("error parsing configuration: %w", err)
	}
	if conf.Network.Type == "" {
		return nil, fmt.Errorf("error parsing configuration: missing 'type'")
	}
	return conf, nil
}

// Given a path to a directory containing a network configuration, and the name of a network,
// loads all plugin definitions found at path `networkConfPath/networkName/*.conf`
func NetworkPluginConfsFromFiles(networkConfPath, networkName string) ([]*PluginConfig, error) {
	var pConfs []*PluginConfig

	pluginConfPath := filepath.Join(networkConfPath, networkName)

	pluginConfFiles, err := ConfFiles(pluginConfPath, []string{".conf"})
	if err != nil {
		return nil, fmt.Errorf("failed to read plugin config files in %s: %w", pluginConfPath, err)
	}

	for _, pluginConfFile := range pluginConfFiles {
		pluginConfBytes, err := os.ReadFile(pluginConfFile)
		if err != nil {
			return nil, fmt.Errorf("error reading %s: %w", pluginConfFile, err)
		}
		pluginConf, err := NetworkPluginConfFromBytes(pluginConfBytes)
		if err != nil {
			return nil, err
		}
		pConfs = append(pConfs, pluginConf)
	}
	return pConfs, nil
}

func NetworkConfFromBytes(confBytes []byte) (*NetworkConfigList, error) {
	rawList := make(map[string]interface{})
	if err := json.Unmarshal(confBytes, &rawList); err != nil {
		return nil, fmt.Errorf("error parsing configuration list: %w", err)
	}

	rawName, ok := rawList["name"]
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: no name")
	}
	name, ok := rawName.(string)
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: invalid name type %T", rawName)
	}

	var cniVersion string
	rawVersion, ok := rawList["cniVersion"]
	if ok {
		cniVersion, ok = rawVersion.(string)
		if !ok {
			return nil, fmt.Errorf("error parsing configuration list: invalid cniVersion type %T", rawVersion)
		}
	}

	rawVersions, ok := rawList["cniVersions"]
	if ok {
		// Parse the current package CNI version
		rvs, ok := rawVersions.([]interface{})
		if !ok {
			return nil, fmt.Errorf("error parsing configuration list: invalid type for cniVersions: %T", rvs)
		}
		vs := make([]string, 0, len(rvs))
		for i, rv := range rvs {
			v, ok := rv.(string)
			if !ok {
				return nil, fmt.Errorf("error parsing configuration list: invalid type for cniVersions index %d: %T", i, rv)
			}
			gt, err := version.GreaterThan(v, version.Current())
			if err != nil {
				return nil, fmt.Errorf("error parsing configuration list: invalid cniVersions entry %s at index %d: %w", v, i, err)
			} else if !gt {
				// Skip versions "greater" than this implementation of the spec
				vs = append(vs, v)
			}
		}

		// if cniVersion was already set, append it to the list for sorting.
		if cniVersion != "" {
			gt, err := version.GreaterThan(cniVersion, version.Current())
			if err != nil {
				return nil, fmt.Errorf("error parsing configuration list: invalid cniVersion %s: %w", cniVersion, err)
			} else if !gt {
				// ignore any versions higher than the current implemented spec version
				vs = append(vs, cniVersion)
			}
		}
		slices.SortFunc[[]string](vs, func(v1, v2 string) int {
			if v1 == v2 {
				return 0
			}
			if gt, _ := version.GreaterThan(v1, v2); gt {
				return 1
			}
			return -1
		})
		if len(vs) > 0 {
			cniVersion = vs[len(vs)-1]
		}
	}

	readBool := func(key string) (bool, error) {
		rawVal, ok := rawList[key]
		if !ok {
			return false, nil
		}
		if b, ok := rawVal.(bool); ok {
			return b, nil
		}

		s, ok := rawVal.(string)
		if !ok {
			return false, fmt.Errorf("error parsing configuration list: invalid type %T for %s", rawVal, key)
		}
		s = strings.ToLower(s)
		switch s {
		case "false":
			return false, nil
		case "true":
			return true, nil
		}
		return false, fmt.Errorf("error parsing configuration list: invalid value %q for %s", s, key)
	}

	disableCheck, err := readBool("disableCheck")
	if err != nil {
		return nil, err
	}

	disableGC, err := readBool("disableGC")
	if err != nil {
		return nil, err
	}

	loadOnlyInlinedPlugins, err := readBool("loadOnlyInlinedPlugins")
	if err != nil {
		return nil, err
	}

	list := &NetworkConfigList{
		Name:                   name,
		DisableCheck:           disableCheck,
		DisableGC:              disableGC,
		LoadOnlyInlinedPlugins: loadOnlyInlinedPlugins,
		CNIVersion:             cniVersion,
		Bytes:                  confBytes,
	}

	var plugins []interface{}
	plug, ok := rawList["plugins"]
	// We can have a `plugins` list key in the main conf,
	// We can also have `loadOnlyInlinedPlugins == true`
	//
	// If `plugins` is there, then `loadOnlyInlinedPlugins` can be true
	//
	// If plugins is NOT there, then `loadOnlyInlinedPlugins` cannot be true
	//
	// We have to have at least some plugins.
	if !ok && loadOnlyInlinedPlugins {
		return nil, fmt.Errorf("error parsing configuration list: `loadOnlyInlinedPlugins` is true, and no 'plugins' key")
	} else if !ok && !loadOnlyInlinedPlugins {
		return list, nil
	}

	plugins, ok = plug.([]interface{})
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: invalid 'plugins' type %T", plug)
	}
	if len(plugins) == 0 {
		return nil, fmt.Errorf("error parsing configuration list: no plugins in list")
	}

	for i, conf := range plugins {
		newBytes, err := json.Marshal(conf)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal plugin config %d: %w", i, err)
		}
		netConf, err := ConfFromBytes(newBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse plugin config %d: %w", i, err)
		}
		list.Plugins = append(list.Plugins, netConf)
	}
	return list, nil
}

func NetworkConfFromFile(filename string) (*NetworkConfigList, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}

	conf, err := NetworkConfFromBytes(bytes)
	if err != nil {
		return nil, err
	}

	if !conf.LoadOnlyInlinedPlugins {
		plugins, err := NetworkPluginConfsFromFiles(filepath.Dir(filename), conf.Name)
		if err != nil {
			return nil, err
		}
		conf.Plugins = append(conf.Plugins, plugins...)
	}

	if len(conf.Plugins) == 0 {
		// Having 0 plugins for a given network is not necessarily a problem,
		// but return as error for caller to decide, since they tried to load
		return nil, fmt.Errorf("no plugin configs found")
	}
	return conf, nil
}

// Deprecated: This file format is no longer supported, use NetworkConfXXX and NetworkPluginXXX functions
func ConfFromBytes(bytes []byte) (*NetworkConfig, error) {
	return NetworkPluginConfFromBytes(bytes)
}

// Deprecated: This file format is no longer supported, use NetworkConfXXX and NetworkPluginXXX functions
func ConfFromFile(filename string) (*NetworkConfig, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}
	return ConfFromBytes(bytes)
}

func ConfListFromBytes(bytes []byte) (*NetworkConfigList, error) {
	return NetworkConfFromBytes(bytes)
}

func ConfListFromFile(filename string) (*NetworkConfigList, error) {
	return NetworkConfFromFile(filename)
}

// ConfFiles simply returns a slice of all files in the provided directory
// with extensions matching the provided set.
func ConfFiles(dir string, extensions []string) ([]string, error) {
	// In part, adapted from rkt/networking/podenv.go#listFiles
	files, err := os.ReadDir(dir)
	switch {
	case err == nil: // break
	case os.IsNotExist(err):
		// If folder not there, return no error - only return an
		// error if we cannot read contents or there are no contents.
		return nil, nil
	default:
		return nil, err
	}

	confFiles := []string{}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		fileExt := filepath.Ext(f.Name())
		for _, ext := range extensions {
			if fileExt == ext {
				confFiles = append(confFiles, filepath.Join(dir, f.Name()))
			}
		}
	}
	return confFiles, nil
}

// Deprecated: This file format is no longer supported, use NetworkConfXXX and NetworkPluginXXX functions
func LoadConf(dir, name string) (*NetworkConfig, error) {
	files, err := ConfFiles(dir, []string{".conf", ".json"})
	switch {
	case err != nil:
		return nil, err
	case len(files) == 0:
		return nil, NoConfigsFoundError{Dir: dir}
	}
	sort.Strings(files)

	for _, confFile := range files {
		conf, err := ConfFromFile(confFile)
		if err != nil {
			return nil, err
		}
		if conf.Network.Name == name {
			return conf, nil
		}
	}
	return nil, NotFoundError{dir, name}
}

func LoadConfList(dir, name string) (*NetworkConfigList, error) {
	return LoadNetworkConf(dir, name)
}

// LoadNetworkConf looks at all the network configs in a given dir,
// loads and parses them all, and returns the first one with an extension of `.conf`
// that matches the provided network name predicate.
func LoadNetworkConf(dir, name string) (*NetworkConfigList, error) {
	// TODO this .conflist/.conf extension thing is confusing and inexact
	// for implementors. We should pick one extension for everything and stick with it.
	files, err := ConfFiles(dir, []string{".conflist"})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	for _, confFile := range files {
		conf, err := NetworkConfFromFile(confFile)
		if err != nil {
			return nil, err
		}
		if conf.Name == name {
			return conf, nil
		}
	}

	// Deprecated: Try and load a network configuration file (instead of list)
	// from the same name, then upconvert.
	singleConf, err := LoadConf(dir, name)
	if err != nil {
		// A little extra logic so the error makes sense
		var ncfErr NoConfigsFoundError
		if len(files) != 0 && errors.As(err, &ncfErr) {
			// Config lists found but no config files found
			return nil, NotFoundError{dir, name}
		}

		return nil, err
	}
	return ConfListFromConf(singleConf)
}

// InjectConf takes a PluginConfig and inserts additional values into it, ensuring the result is serializable.
func InjectConf(original *PluginConfig, newValues map[string]interface{}) (*PluginConfig, error) {
	config := make(map[string]interface{})
	err := json.Unmarshal(original.Bytes, &config)
	if err != nil {
		return nil, fmt.Errorf("unmarshal existing network bytes: %w", err)
	}

	for key, value := range newValues {
		if key == "" {
			return nil, fmt.Errorf("keys cannot be empty")
		}

		if value == nil {
			return nil, fmt.Errorf("key '%s' value must not be nil", key)
		}

		config[key] = value
	}

	newBytes, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	return NetworkPluginConfFromBytes(newBytes)
}

// ConfListFromConf "upconverts" a network config in to a NetworkConfigList,
// with the single network as the only entry in the list.
//
// Deprecated: Non-conflist file formats are unsupported, use NetworkConfXXX and NetworkPluginXXX functions
func ConfListFromConf(original *PluginConfig) (*NetworkConfigList, error) {
	// Re-deserialize the config's json, then make a raw map configlist.
	// This may seem a bit strange, but it's to make the Bytes fields
	// actually make sense. Otherwise, the generated json is littered with
	// golang default values.

	rawConfig := make(map[string]interface{})
	if err := json.Unmarshal(original.Bytes, &rawConfig); err != nil {
		return nil, err
	}

	rawConfigList := map[string]interface{}{
		"name":       original.Network.Name,
		"cniVersion": original.Network.CNIVersion,
		"plugins":    []interface{}{rawConfig},
	}

	b, err := json.Marshal(rawConfigList)
	if err != nil {
		return nil, err
	}
	return ConfListFromBytes(b)
}
