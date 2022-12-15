// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tracetest // import "go.opentelemetry.io/otel/sdk/trace/tracetest"

import (
	"context"
	"sync"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// SpanRecorder records started and ended spans.
type SpanRecorder struct {
	startedMu sync.RWMutex
	started   []sdktrace.ReadWriteSpan

	endedMu sync.RWMutex
	ended   []sdktrace.ReadOnlySpan
}

var _ sdktrace.SpanProcessor = (*SpanRecorder)(nil)

func NewSpanRecorder() *SpanRecorder {
	return new(SpanRecorder)
}

// OnStart records started spans.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) OnStart(_ context.Context, s sdktrace.ReadWriteSpan) {
	sr.startedMu.Lock()
	defer sr.startedMu.Unlock()
	sr.started = append(sr.started, s)
}

// OnEnd records completed spans.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) OnEnd(s sdktrace.ReadOnlySpan) {
	sr.endedMu.Lock()
	defer sr.endedMu.Unlock()
	sr.ended = append(sr.ended, s)
}

// Shutdown does nothing.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) Shutdown(context.Context) error {
	return nil
}

// ForceFlush does nothing.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) ForceFlush(context.Context) error {
	return nil
}

// Started returns a copy of all started spans that have been recorded.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) Started() []sdktrace.ReadWriteSpan {
	sr.startedMu.RLock()
	defer sr.startedMu.RUnlock()
	dst := make([]sdktrace.ReadWriteSpan, len(sr.started))
	copy(dst, sr.started)
	return dst
}

// Ended returns a copy of all ended spans that have been recorded.
//
// This method is safe to be called concurrently.
func (sr *SpanRecorder) Ended() []sdktrace.ReadOnlySpan {
	sr.endedMu.RLock()
	defer sr.endedMu.RUnlock()
	dst := make([]sdktrace.ReadOnlySpan, len(sr.ended))
	copy(dst, sr.ended)
	return dst
}
