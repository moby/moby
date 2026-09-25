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

package tracing

import "github.com/containerd/log/otel"

// Deprecated: use [otel.HookOpt] instead.
//
//go:fix inline
type HookOpt = otel.HookOpt

// NewLogrusHook creates a new logrus hook
//
// Deprecated: use [otel.NewLogrusHook] instead.
//
//go:fix inline
func NewLogrusHook(opts ...otel.HookOpt) *otel.LogrusHook {
	return otel.NewLogrusHook(opts...)
}

// Deprecated: use [otel.WithTraceIDField] instead.
//
//go:fix inline
func WithTraceIDField(enabled bool) otel.HookOpt {
	return otel.WithTraceIDField(enabled)
}

// LogrusHook is a [logrus.Hook] which adds logrus events to active spans.
// If the span is not recording or the span context is invalid, the hook
// is a no-op.
//
// Deprecated: use [otel.LogrusHook] instead.
//
// [logrus.Hook]: https://github.com/sirupsen/logrus/blob/v1.9.3/hooks.go#L3-L11
//
//go:fix inline
type LogrusHook = otel.LogrusHook
