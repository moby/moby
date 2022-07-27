// Copyright 2016 CNI authors
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

package version

import (
	"encoding/json"
	"fmt"

	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/cni/pkg/types/create"
)

// Current reports the version of the CNI spec implemented by this library
func Current() string {
	return types100.ImplementedSpecVersion
}

// Legacy PluginInfo describes a plugin that is backwards compatible with the
// CNI spec version 0.1.0.  In particular, a runtime compiled against the 0.1.0
// library ought to work correctly with a plugin that reports support for
// Legacy versions.
//
// Any future CNI spec versions which meet this definition should be added to
// this list.
var Legacy = PluginSupports("0.1.0", "0.2.0")
var All = PluginSupports("0.1.0", "0.2.0", "0.3.0", "0.3.1", "0.4.0", "1.0.0")

// VersionsFrom returns a list of versions starting from min, inclusive
func VersionsStartingFrom(min string) PluginInfo {
	out := []string{}
	// cheat, just assume ordered
	ok := false
	for _, v := range All.SupportedVersions() {
		if !ok && v == min {
			ok = true
		}
		if ok {
			out = append(out, v)
		}
	}
	return PluginSupports(out...)
}

// Finds a Result object matching the requested version (if any) and asks
// that object to parse the plugin result, returning an error if parsing failed.
func NewResult(version string, resultBytes []byte) (types.Result, error) {
	return create.Create(version, resultBytes)
}

// ParsePrevResult parses a prevResult in a NetConf structure and sets
// the NetConf's PrevResult member to the parsed Result object.
func ParsePrevResult(conf *types.NetConf) error {
	if conf.RawPrevResult == nil {
		return nil
	}

	// Prior to 1.0.0, Result types may not marshal a CNIVersion. Since the
	// result version must match the config version, if the Result's version
	// is empty, inject the config version.
	if ver, ok := conf.RawPrevResult["CNIVersion"]; !ok || ver == "" {
		conf.RawPrevResult["CNIVersion"] = conf.CNIVersion
	}

	resultBytes, err := json.Marshal(conf.RawPrevResult)
	if err != nil {
		return fmt.Errorf("could not serialize prevResult: %w", err)
	}

	conf.RawPrevResult = nil
	conf.PrevResult, err = create.Create(conf.CNIVersion, resultBytes)
	if err != nil {
		return fmt.Errorf("could not parse prevResult: %w", err)
	}

	return nil
}
