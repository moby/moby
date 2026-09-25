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

package shim

// This file contains the compatibility layer between the new shim bootstrap
// protocol (see https://github.com/containerd/containerd/pull/12786) and the
// old shim APIs (prior containerd 2.3), which mainly relies on CLI, env vars, stdin, and spec.json annotations.
// Once settled, this file should be removed.

import (
	"errors"
	"fmt"

	bootapi "github.com/containerd/containerd/api/runtime/bootstrap/v1"
	"github.com/containerd/containerd/v2/pkg/protobuf/proto"
)

var errDeprecatedBootstrapAPI = errors.New("shim was started through the deprecated API but was built against the new API; if you upgraded containerd, you may need to restart the daemon")

// parseBootstrapParams parses input from a caller using the bootstrap API.
// Legacy runtime options can unmarshal as BootstrapParams, so verify the
// identity against the command-line values.
func parseBootstrapParams(input []byte, cliID, cliNamespace string) (*bootapi.BootstrapParams, error) {
	params := &bootapi.BootstrapParams{}
	if len(input) == 0 {
		return nil, errDeprecatedBootstrapAPI
	}
	if err := proto.Unmarshal(input, params); err != nil {
		return nil, fmt.Errorf("%w: failed to unmarshal bootstrap parameters: %w", errDeprecatedBootstrapAPI, err)
	}
	if params.InstanceID != cliID || params.Namespace != cliNamespace {
		return nil, fmt.Errorf("bootstrap parameters do not match command-line arguments: %w", errDeprecatedBootstrapAPI)
	}
	return params, nil
}
