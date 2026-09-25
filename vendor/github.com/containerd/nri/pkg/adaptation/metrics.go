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

import "time"

// Metrics defines the interface that a consumer can implement to collect
// and emit metrics regarding NRI plugin activity.
// Implementations of this interface must be thread-safe, as these methods
// are invoked concurrently across multiple goroutines handling plugin requests.
type Metrics interface {
	// RecordPluginInvocation records the invocation of a plugin for a specific operation.
	RecordPluginInvocation(pluginName, operation string, err error)

	// RecordPluginLatency records the latency of a plugin invocation.
	RecordPluginLatency(pluginName, operation string, latency time.Duration)

	// RecordPluginAdjustments records the adjustments returned by a plugin.
	RecordPluginAdjustments(pluginName, operation string, adjust *ContainerAdjustment, updates, evicts int)

	// UpdatePluginCount sets the number of currently active plugins.
	UpdatePluginCount(count int)
}

// noopMetrics provides a default, no-operation implementation of the Metrics interface.
type noopMetrics struct{}

var _ Metrics = (*noopMetrics)(nil)

func (n *noopMetrics) RecordPluginInvocation(_, _ string, _ error)                           {}
func (n *noopMetrics) RecordPluginLatency(_, _ string, _ time.Duration)                      {}
func (n *noopMetrics) RecordPluginAdjustments(_, _ string, _ *ContainerAdjustment, _, _ int) {}
func (n *noopMetrics) UpdatePluginCount(_ int)                                               {}
