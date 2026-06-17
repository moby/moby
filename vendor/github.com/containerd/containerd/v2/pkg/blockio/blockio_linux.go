//go:build linux

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

package blockio

import (
	"fmt"
	"sync"

	"github.com/intel/goresctrl/pkg/blockio"
	runtimespec "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/containerd/log"
)

var (
	enabled   bool
	enabledMu sync.RWMutex
)

// IsEnabled checks whether blockio is enabled.
func IsEnabled() bool {
	enabledMu.RLock()
	defer enabledMu.RUnlock()

	return enabled
}

// SetConfig updates blockio config with a given config path.
func SetConfig(configFilePath string) error {
	enabledMu.Lock()
	defer enabledMu.Unlock()

	enabled = false
	if configFilePath == "" {
		log.L.Debug("No blockio config file specified, blockio not configured")
		return nil
	}

	if err := blockio.SetConfigFromFile(configFilePath, true); err != nil {
		return fmt.Errorf("blockio not enabled: %w", err)
	}
	enabled = true
	return nil
}

// ClassNameToLinuxOCI converts blockio class name into the LinuxBlockIO
// structure in the OCI runtime spec.
func ClassNameToLinuxOCI(className string) (*runtimespec.LinuxBlockIO, error) {
	return blockio.OciLinuxBlockIO(className)
}

// ContainerClassFromAnnotations examines container and pod annotations of a
// container and returns its blockio class.
func ContainerClassFromAnnotations(containerName string, containerAnnotations, podAnnotations map[string]string) (string, error) {
	return blockio.ContainerClassFromAnnotations(containerName, containerAnnotations, podAnnotations)
}
