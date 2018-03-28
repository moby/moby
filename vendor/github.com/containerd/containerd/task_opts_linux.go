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

package containerd

import (
	"context"

	"github.com/opencontainers/runtime-spec/specs-go"
)

// WithResources sets the provided resources for task updates
func WithResources(resources *specs.LinuxResources) UpdateTaskOpts {
	return func(ctx context.Context, client *Client, r *UpdateTaskInfo) error {
		r.Resources = resources
		return nil
	}
}
