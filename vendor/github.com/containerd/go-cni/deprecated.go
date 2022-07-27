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

package cni

import types100 "github.com/containernetworking/cni/pkg/types/100"

// Deprecated: use cni.Opt instead
type CNIOpt = Opt //revive:disable // type name will be used as cni.CNIOpt by other packages, and that stutters

// Deprecated: use cni.Result instead
type CNIResult = Result //revive:disable // type name will be used as cni.CNIResult by other packages, and that stutters

// GetCNIResultFromResults creates a Result from the given slice of types100.Result,
// adding structured data containing the interface configuration for each of the
// interfaces created in the namespace. It returns an error if validation of
// results fails, or if a network could not be found.
// Deprecated: do not use
func (c *libcni) GetCNIResultFromResults(results []*types100.Result) (*Result, error) {
	return c.createResult(results)
}
