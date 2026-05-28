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
	"maps"
	"slices"

	"github.com/containerd/nri/pkg/version"
)

const (
	// UnknownVersion is an alias of version.UnknownVersion.
	UnknownVersion = version.UnknownVersion
	// DevelVersion is an alias for version.DevelVersion.
	DevelVersion = version.DevelVersion
)

// InferVersionFromRuntime tries to infer the version of NRI used in the runtime.
func InferVersionFromRuntime(runtime, runtimeVersion string) (string, error) {
	versionMap := runtimeVersionMap[runtime]
	if versionMap == nil {
		return UnknownVersion, fmt.Errorf("can't infer version for unknown runtime %q", runtime)
	}

	tag := version.FindClosestMatch(runtimeVersion, slices.Collect(maps.Keys(versionMap)))
	if tag == "" {
		return UnknownVersion, fmt.Errorf("failed to find latest version for %q", runtimeVersion)
	}

	nriVersion := versionMap[tag]
	if nriVersion == "" {
		return UnknownVersion, fmt.Errorf("unknown runtime version %q %q", runtime, runtimeVersion)
	}

	return nriVersion, nil
}

var (
	runtimeVersionMap = map[string]map[string]string{
		"containerd": {
			"v1.7.0":  "v0.3.0",
			"v1.7.1":  "v0.3.0",
			"v1.7.2":  "v0.3.0",
			"v1.7.3":  "v0.3.0",
			"v1.7.4":  "v0.3.0",
			"v1.7.5":  "v0.3.0",
			"v1.7.6":  "v0.3.0",
			"v1.7.7":  "v0.4.0",
			"v1.7.8":  "v0.4.0",
			"v1.7.9":  "v0.4.0",
			"v1.7.10": "v0.4.0",
			"v1.7.11": "v0.4.0",
			"v1.7.12": "v0.4.0",
			"v1.7.13": "v0.4.0",
			"v1.7.14": "v0.6.0",
			"v1.7.15": "v0.6.0",
			"v1.7.16": "v0.6.1",
			"v1.7.17": "v0.6.1",
			"v1.7.18": "v0.6.1",
			"v1.7.19": "v0.6.1",
			"v1.7.20": "v0.6.1",
			"v1.7.21": "v0.6.1",
			"v1.7.22": "v0.6.1",
			"v1.7.23": "v0.6.1",
			"v1.7.24": "v0.6.1",
			"v1.7.25": "v0.6.1",
			"v1.7.26": "v0.8.0",
			"v1.7.27": "v0.8.0",
			"v1.7.28": "v0.8.0",
			"v1.7.29": "v0.8.0",
			"v1.7.30": "v0.8.0",
			"v2.0.0":  "v0.8.0",
			"v2.0.1":  "v0.8.0",
			"v2.0.2":  "v0.8.0",
			"v2.0.3":  "v0.8.0",
			"v2.0.4":  "v0.8.0",
			"v2.0.5":  "v0.8.0",
			"v2.0.6":  "v0.8.0",
			"v2.0.7":  "v0.8.0",
			"v2.1.0":  "v0.8.0",
			"v2.1.1":  "v0.8.0",
			"v2.1.2":  "v0.8.0",
			"v2.1.3":  "v0.8.0",
			"v2.1.4":  "v0.8.0",
			"v2.1.5":  "v0.8.0",
			"v2.1.6":  "v0.8.0",
			"v2.2.0":  "v0.10.0",
			"v2.2.1":  "v0.11.0",
		},
		"cri-o": {
			"v1.26.0":  "v0.2.0",
			"v1.26.1":  "v0.2.0",
			"v1.26.2":  "v0.3.0",
			"v1.26.3":  "v0.3.0",
			"v1.26.4":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.0":  "v0.3.0",
			"v1.27.1":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.2":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.3":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.4":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.5":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.6":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.7":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.27.8":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.0":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.1":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.2":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.3":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.4":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.5":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.6":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.7":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.8":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.9":  "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.10": "v0.3.1-0.20230504231226-94185418e253",
			"v1.28.11": "v0.3.1-0.20230504231226-94185418e253",
			"v1.29.0":  "v0.5.0",
			"v1.29.1":  "v0.5.0",
			"v1.29.2":  "v0.6.0",
			"v1.29.3":  "v0.6.0",
			"v1.29.4":  "v0.6.0",
			"v1.29.5":  "v0.6.0",
			"v1.29.6":  "v0.6.0",
			"v1.29.7":  "v0.6.0",
			"v1.29.8":  "v0.6.0",
			"v1.29.9":  "v0.6.0",
			"v1.29.10": "v0.6.0",
			"v1.29.11": "v0.6.0",
			"v1.29.12": "v0.6.0",
			"v1.29.13": "v0.6.0",
			"v1.30.0":  "v0.6.0",
			"v1.30.1":  "v0.6.0",
			"v1.30.2":  "v0.6.0",
			"v1.30.3":  "v0.6.0",
			"v1.30.4":  "v0.6.0",
			"v1.30.5":  "v0.6.0",
			"v1.30.6":  "v0.6.0",
			"v1.30.7":  "v0.6.0",
			"v1.30.8":  "v0.6.0",
			"v1.30.9":  "v0.6.0",
			"v1.30.10": "v0.6.0",
			"v1.30.11": "v0.6.0",
			"v1.30.12": "v0.6.0",
			"v1.30.13": "v0.6.0",
			"v1.30.14": "v0.6.0",
			"v1.31.0":  "v0.6.1",
			"v1.31.1":  "v0.6.1",
			"v1.31.2":  "v0.6.1",
			"v1.31.3":  "v0.6.1",
			"v1.31.4":  "v0.6.1",
			"v1.31.5":  "v0.6.1",
			"v1.31.6":  "v0.6.1",
			"v1.31.7":  "v0.6.1",
			"v1.31.8":  "v0.6.1",
			"v1.31.9":  "v0.6.1",
			"v1.31.10": "v0.6.1",
			"v1.31.11": "v0.6.1",
			"v1.31.12": "v0.6.1",
			"v1.31.13": "v0.6.1",
			"v1.32.0":  "v0.9.0",
			"v1.32.1":  "v0.9.0",
			"v1.32.2":  "v0.9.0",
			"v1.32.3":  "v0.9.0",
			"v1.32.4":  "v0.9.0",
			"v1.32.5":  "v0.9.0",
			"v1.32.6":  "v0.9.0",
			"v1.32.7":  "v0.9.0",
			"v1.32.8":  "v0.9.0",
			"v1.32.9":  "v0.9.0",
			"v1.32.10": "v0.9.0",
			"v1.32.11": "v0.9.0",
			"v1.32.12": "v0.9.0",
			"v1.33.0":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.1":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.2":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.3":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.4":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.5":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.6":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.7":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.33.8":  "v0.9.1-0.20250219030224-e597e7849f24",
			"v1.34.0":  "v0.10.0",
			"v1.34.1":  "v0.10.0",
			"v1.34.2":  "v0.10.0",
			"v1.34.3":  "v0.10.0",
			"v1.34.4":  "v0.10.0",
			"v1.35.0":  "v0.11.0",
		},
	}
)
