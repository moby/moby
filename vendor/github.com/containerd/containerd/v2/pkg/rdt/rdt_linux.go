//go:build linux && !no_rdt

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

package rdt

import (
	"fmt"
	"sync"

	"github.com/containerd/log"

	"github.com/intel/goresctrl/pkg/rdt"
)

const (
	// ResctrlPrefix is the prefix used for class/closid directories under the resctrl filesystem
	ResctrlPrefix = ""
)

var (
	enabled   bool
	enabledMu sync.RWMutex
)

// IsEnabled checks whether rdt is enabled.
func IsEnabled() bool {
	enabledMu.RLock()
	defer enabledMu.RUnlock()

	return enabled
}

var (
	initOnce sync.Once
	initErr  error
)

// SetConfig updates rdt config with a given config path.
func SetConfig(configFilePath string) error {
	enabledMu.Lock()
	defer enabledMu.Unlock()

	enabled = false
	if configFilePath == "" {
		log.L.Debug("No RDT config file specified, RDT not configured")
		return nil
	}

	initOnce.Do(func() {
		err := rdt.Initialize(ResctrlPrefix)
		if err != nil {
			initErr = fmt.Errorf("RDT not enabled: %w", err)
		}
	})
	if initErr != nil {
		return initErr
	}

	if err := rdt.SetConfigFromFile(configFilePath, true); err != nil {
		return err
	}
	enabled = true
	return nil
}

// ContainerClassFromAnnotations examines container and pod annotations of a
// container and returns its RDT class.
func ContainerClassFromAnnotations(containerName string, containerAnnotations, podAnnotations map[string]string) (string, error) {
	return rdt.ContainerClassFromAnnotations(containerName, containerAnnotations, podAnnotations)
}
