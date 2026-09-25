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

package oci

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

var possibleCPUsParsed = sync.OnceValues(func() ([]int, error) {
	data, err := os.ReadFile("/sys/devices/system/cpu/possible")
	if err != nil {
		return nil, err
	}
	return parsePossibleCPUs(strings.TrimSpace(string(data)))
})

func appendCPUThrottlePaths(paths []string, cpus []int) []string {
	for _, cpu := range cpus {
		path := fmt.Sprintf("/sys/devices/system/cpu/cpu%d/thermal_throttle", cpu)
		if _, err := os.Stat(path); err == nil {
			paths = append(paths, path)
		}
	}
	return paths
}
