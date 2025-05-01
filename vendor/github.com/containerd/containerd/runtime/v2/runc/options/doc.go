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

package options

import api "github.com/containerd/containerd/api/types/runc/options"

// Options is a type alias of github.com/containerd/containerd/api/types/runc/options.Options
//
// Deprecated: use [api.Options] instead
type Options = api.Options

// CheckpointOptions is a type alias of github.com/containerd/containerd/api/types/runc/options.CheckpointOptions
//
// Deprecated: use [api.CheckpointOptions] instead
type CheckpointOptions = api.CheckpointOptions

// ProcessDetails is a type alias of github.com/containerd/containerd/api/types/runc/options.ProcessDetails
//
// Deprecated: use [api.ProcessDetails] instead
type ProcessDetails = api.ProcessDetails

//nolint:revive
var File_github_com_containerd_containerd_runtime_v2_runc_options_oci_proto = api.File_github_com_containerd_containerd_api_types_runc_options_oci_proto
