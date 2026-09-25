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
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// FromOCILinuxNamespaces returns a namespace slice from an OCI runtime Spec.
func FromOCILinuxNamespaces(o []rspec.LinuxNamespace) []*LinuxNamespace {
	var namespaces []*LinuxNamespace
	for _, ns := range o {
		namespaces = append(namespaces, &LinuxNamespace{
			Type: string(ns.Type),
			Path: ns.Path,
		})
	}
	return namespaces
}

// IsMarkedForRemoval checks if a LinuxNamespace is marked for removal.
func (n *LinuxNamespace) IsMarkedForRemoval() (string, bool) {
	return IsMarkedForRemoval(n.Type)
}
