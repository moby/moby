// +build windows

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

package archive

// ApplyOptions provides additional options for an Apply operation
type ApplyOptions struct {
	ParentLayerPaths        []string // Parent layer paths used for Windows layer apply
	IsWindowsContainerLayer bool     // True if the tar stream to be applied is a Windows Container Layer
	Filter                  Filter   // Filter tar headers
}

// WithParentLayers adds parent layers to the apply process this is required
// for all Windows layers except the base layer.
func WithParentLayers(parentPaths []string) ApplyOpt {
	return func(options *ApplyOptions) error {
		options.ParentLayerPaths = parentPaths
		return nil
	}
}

// AsWindowsContainerLayer indicates that the tar stream to apply is that of
// a Windows Container Layer. The caller must be holding SeBackupPrivilege and
// SeRestorePrivilege.
func AsWindowsContainerLayer() ApplyOpt {
	return func(options *ApplyOptions) error {
		options.IsWindowsContainerLayer = true
		return nil
	}
}
