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

package adaptation

import (
	"context"
	"fmt"
)

// DeprecationRecorder is the interface for recording NRI deprecation warnings.
type DeprecationRecorder interface {
	// PluginWarning records a warning for a plugin.
	PluginWarning(ctx context.Context, d Deprecation, plugin, details string)
}

// Deprecation is the type for NRI deprecations.
type Deprecation int

const (
	// DeprecatedStateChange indicates that a plugin does not implement per
	// request RPC calls, using the deprecated StateChange instead.
	DeprecatedStateChange Deprecation = iota + 1
)

func (d Deprecation) String() string {
	switch d {
	case DeprecatedStateChange:
		return "deprecated StateChange"
	default:
		return fmt.Sprintf("unknown deprecation (%d)", d)
	}
}
