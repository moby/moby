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

package trace

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/trace"
)

// snapshot is an record of a spans state at a particular checkpointed time.
// It is used as a read-only representation of that state.
type snapshot struct {
	name                   string
	spanContext            trace.SpanContext
	parent                 trace.SpanContext
	spanKind               trace.SpanKind
	startTime              time.Time
	endTime                time.Time
	attributes             []attribute.KeyValue
	events                 []Event
	links                  []trace.Link
	status                 Status
	childSpanCount         int
	droppedAttributeCount  int
	droppedEventCount      int
	droppedLinkCount       int
	resource               *resource.Resource
	instrumentationLibrary instrumentation.Library
}

var _ ReadOnlySpan = snapshot{}

func (s snapshot) private() {}

// Name returns the name of the span.
func (s snapshot) Name() string {
	return s.name
}

// SpanContext returns the unique SpanContext that identifies the span.
func (s snapshot) SpanContext() trace.SpanContext {
	return s.spanContext
}

// Parent returns the unique SpanContext that identifies the parent of the
// span if one exists. If the span has no parent the returned SpanContext
// will be invalid.
func (s snapshot) Parent() trace.SpanContext {
	return s.parent
}

// SpanKind returns the role the span plays in a Trace.
func (s snapshot) SpanKind() trace.SpanKind {
	return s.spanKind
}

// StartTime returns the time the span started recording.
func (s snapshot) StartTime() time.Time {
	return s.startTime
}

// EndTime returns the time the span stopped recording. It will be zero if
// the span has not ended.
func (s snapshot) EndTime() time.Time {
	return s.endTime
}

// Attributes returns the defining attributes of the span.
func (s snapshot) Attributes() []attribute.KeyValue {
	return s.attributes
}

// Links returns all the links the span has to other spans.
func (s snapshot) Links() []trace.Link {
	return s.links
}

// Events returns all the events that occurred within in the spans
// lifetime.
func (s snapshot) Events() []Event {
	return s.events
}

// Status returns the spans status.
func (s snapshot) Status() Status {
	return s.status
}

// InstrumentationLibrary returns information about the instrumentation
// library that created the span.
func (s snapshot) InstrumentationLibrary() instrumentation.Library {
	return s.instrumentationLibrary
}

// Resource returns information about the entity that produced the span.
func (s snapshot) Resource() *resource.Resource {
	return s.resource
}

// DroppedAttributes returns the number of attributes dropped by the span
// due to limits being reached.
func (s snapshot) DroppedAttributes() int {
	return s.droppedAttributeCount
}

// DroppedLinks returns the number of links dropped by the span due to limits
// being reached.
func (s snapshot) DroppedLinks() int {
	return s.droppedLinkCount
}

// DroppedEvents returns the number of events dropped by the span due to
// limits being reached.
func (s snapshot) DroppedEvents() int {
	return s.droppedEventCount
}

// ChildSpanCount returns the count of spans that consider the span a
// direct parent.
func (s snapshot) ChildSpanCount() int {
	return s.childSpanCount
}
