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

func ConfFromBytes(bytes []byte) (*NetworkConfig, error) {
	conf := &NetworkConfig{Bytes: bytes, Network: &types.NetConf{}}
	if err := json.Unmarshal(bytes, conf.Network); err != nil {
		return nil, fmt.Errorf("error parsing configuration: %w", err)
	}
	if conf.Network.Type == "" {
		return nil, fmt.Errorf("error parsing configuration: missing 'type'")
	}
	return conf, nil
}

func ConfFromFile(filename string) (*NetworkConfig, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}
	return ConfFromBytes(bytes)
}

func ConfListFromBytes(bytes []byte) (*NetworkConfigList, error) {
	rawList := make(map[string]interface{})
	if err := json.Unmarshal(bytes, &rawList); err != nil {
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

	list := &NetworkConfigList{
		Name:         name,
		DisableCheck: disableCheck,
		DisableGC:    disableGC,
		CNIVersion:   cniVersion,
		Bytes:        bytes,
	}

	var plugins []interface{}
	plug, ok := rawList["plugins"]
	if !ok {
		return nil, fmt.Errorf("error parsing configuration list: no 'plugins' key")
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

func ConfListFromFile(filename string) (*NetworkConfigList, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading %s: %w", filename, err)
	}
	return ConfListFromBytes(bytes)
}

func ConfFiles(dir string, extensions []string) ([]string, error) {
	// In part, adapted from rkt/networking/podenv.go#listFiles
	files, err := os.ReadDir(dir)
	switch {
	case err == nil: // break
	case os.IsNotExist(err):
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
	files, err := ConfFiles(dir, []string{".conflist"})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)

	for _, confFile := range files {
		conf, err := ConfListFromFile(confFile)
		if err != nil {
			return nil, err
		}
		if conf.Name == name {
			return conf, nil
		}
	}

	// Try and load a network configuration file (instead of list)
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

func InjectConf(original *NetworkConfig, newValues map[string]interface{}) (*NetworkConfig, error) {
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

	return ConfFromBytes(newBytes)
}

// ConfListFromConf "upconverts" a network config in to a NetworkConfigList,
// with the single network as the only entry in the list.
func ConfListFromConf(original *NetworkConfig) (*NetworkConfigList, error) {
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
