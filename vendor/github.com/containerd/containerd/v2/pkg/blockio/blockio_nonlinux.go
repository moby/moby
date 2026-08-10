//go:build !linux

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

import runtimespec "github.com/opencontainers/runtime-spec/specs-go"

// IsEnabled always returns false in non-linux platforms.
func IsEnabled() bool { return false }

// SetConfig always is no-op in non-linux platforms.
func SetConfig(configFilePath string) error {
	return nil
}

// ClassNameToLinuxOCI always is no-op in non-linux platforms.
func ClassNameToLinuxOCI(className string) (*runtimespec.LinuxBlockIO, error) {
	return nil, nil
}

// ContainerClassFromAnnotations always is no-op in non-linux platforms.
func ContainerClassFromAnnotations(containerName string, containerAnnotations, podAnnotations map[string]string) (string, error) {
	return "", nil
}
