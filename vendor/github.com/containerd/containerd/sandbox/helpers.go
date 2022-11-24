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

package sandbox

import (
	"github.com/containerd/containerd/api/types"
	"github.com/containerd/containerd/protobuf"
	gogo_types "github.com/containerd/containerd/protobuf/types"
	"github.com/containerd/typeurl/v2"
)

// ToProto will map Sandbox struct to it's protobuf definition
func ToProto(sandbox *Sandbox) *types.Sandbox {
	extensions := make(map[string]*gogo_types.Any)
	for k, v := range sandbox.Extensions {
		extensions[k] = protobuf.FromAny(v)
	}
	return &types.Sandbox{
		SandboxID: sandbox.ID,
		Runtime: &types.Sandbox_Runtime{
			Name:    sandbox.Runtime.Name,
			Options: protobuf.FromAny(sandbox.Runtime.Options),
		},
		Labels:     sandbox.Labels,
		CreatedAt:  protobuf.ToTimestamp(sandbox.CreatedAt),
		UpdatedAt:  protobuf.ToTimestamp(sandbox.UpdatedAt),
		Extensions: extensions,
		Spec:       protobuf.FromAny(sandbox.Spec),
	}
}

// FromProto map protobuf sandbox definition to Sandbox struct
func FromProto(sandboxpb *types.Sandbox) Sandbox {
	runtime := RuntimeOpts{
		Name:    sandboxpb.Runtime.Name,
		Options: sandboxpb.Runtime.Options,
	}

	extensions := make(map[string]typeurl.Any)
	for k, v := range sandboxpb.Extensions {
		v := v
		extensions[k] = v
	}

	return Sandbox{
		ID:         sandboxpb.SandboxID,
		Labels:     sandboxpb.Labels,
		Runtime:    runtime,
		Spec:       sandboxpb.Spec,
		CreatedAt:  protobuf.FromTimestamp(sandboxpb.CreatedAt),
		UpdatedAt:  protobuf.FromTimestamp(sandboxpb.UpdatedAt),
		Extensions: extensions,
	}
}
