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

package trace // import "go.opentelemetry.io/otel/sdk/trace"

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"

	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/internal"
	"go.opentelemetry.io/otel/sdk/resource"
)

// ReadOnlySpan allows reading information from the data structure underlying a
// trace.Span. It is used in places where reading information from a span is
// necessary but changing the span isn't necessary or allowed.
//
// Warning: methods may be added to this interface in minor releases.
type ReadOnlySpan interface {
	// Name returns the name of the span.
	Name() string
	// SpanContext returns the unique SpanContext that identifies the span.
	SpanContext() trace.SpanContext
	// Parent returns the unique SpanContext that identifies the parent of the
	// span if one exists. If the span has no parent the returned SpanContext
	// will be invalid.
	Parent() trace.SpanContext
	// SpanKind returns the role the span plays in a Trace.
	SpanKind() trace.SpanKind
	// StartTime returns the time the span started recording.
	StartTime() time.Time
	// EndTime returns the time the span stopped recording. It will be zero if
	// the span has not ended.
	EndTime() time.Time
	// Attributes returns the defining attributes of the span.
	Attributes() []attribute.KeyValue
	// Links returns all the links the span has to other spans.
	Links() []trace.Link
	// Events returns all the events that occurred within in the spans
	// lifetime.
	Events() []Event
	// Status returns the spans status.
	Status() Status
	// InstrumentationLibrary returns information about the instrumentation
	// library that created the span.
	InstrumentationLibrary() instrumentation.Library
	// Resource returns information about the entity that produced the span.
	Resource() *resource.Resource
	// DroppedAttributes returns the number of attributes dropped by the span
	// due to limits being reached.
	DroppedAttributes() int
	// DroppedLinks returns the number of links dropped by the span due to
	// limits being reached.
	DroppedLinks() int
	// DroppedEvents returns the number of events dropped by the span due to
	// limits being reached.
	DroppedEvents() int
	// ChildSpanCount returns the count of spans that consider the span a
	// direct parent.
	ChildSpanCount() int

	// A private method to prevent users implementing the
	// interface and so future additions to it will not
	// violate compatibility.
	private()
}

// ReadWriteSpan exposes the same methods as trace.Span and in addition allows
// reading information from the underlying data structure.
// This interface exposes the union of the methods of trace.Span (which is a
// "write-only" span) and ReadOnlySpan. New methods for writing or reading span
// information should be added under trace.Span or ReadOnlySpan, respectively.
//
// Warning: methods may be added to this interface in minor releases.
type ReadWriteSpan interface {
	trace.Span
	ReadOnlySpan
}

// span is an implementation of the OpenTelemetry Span API representing the
// individual component of a trace.
type span struct {
	// mu protects the contents of this span.
	mu sync.Mutex

	// parent holds the parent span of this span as a trace.SpanContext.
	parent trace.SpanContext

	// spanKind represents the kind of this span as a trace.SpanKind.
	spanKind trace.SpanKind

	// name is the name of this span.
	name string

	// startTime is the time at which this span was started.
	startTime time.Time

	// endTime is the time at which this span was ended. It contains the zero
	// value of time.Time until the span is ended.
	endTime time.Time

	// status is the status of this span.
	status Status

	// childSpanCount holds the number of child spans created for this span.
	childSpanCount int

	// resource contains attributes representing an entity that produced this
	// span.
	resource *resource.Resource

	// instrumentationLibrary defines the instrumentation library used to
	// provide instrumentation.
	instrumentationLibrary instrumentation.Library

	// spanContext holds the SpanContext of this span.
	spanContext trace.SpanContext

	// attributes are capped at configured limit. When the capacity is reached
	// an oldest entry is removed to create room for a new entry.
	attributes *attributesMap

	// events are stored in FIFO queue capped by configured limit.
	events *evictedQueue

	// links are stored in FIFO queue capped by configured limit.
	links *evictedQueue

	// executionTracerTaskEnd ends the execution tracer span.
	executionTracerTaskEnd func()

	// tracer is the SDK tracer that created this span.
	tracer *tracer

	// spanLimits holds the limits to this span.
	spanLimits SpanLimits
}

var _ trace.Span = &span{}

// SpanContext returns the SpanContext of this span.
func (s *span) SpanContext() trace.SpanContext {
	if s == nil {
		return trace.SpanContext{}
	}
	return s.spanContext
}

// IsRecording returns if this span is being recorded. If this span has ended
// this will return false.
func (s *span) IsRecording() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	return !s.startTime.IsZero() && s.endTime.IsZero()
}

// SetStatus sets the status of the Span in the form of a code and a
// description, overriding previous values set. The description is only
// included in the set status when the code is for an error. If this span is
// not being recorded than this method does nothing.
func (s *span) SetStatus(code codes.Code, description string) {
	if !s.IsRecording() {
		return
	}

	status := Status{Code: code}
	if code == codes.Error {
		status.Description = description
	}

	s.mu.Lock()
	s.status = status
	s.mu.Unlock()
}

// SetAttributes sets attributes of this span.
//
// If a key from attributes already exists the value associated with that key
// will be overwritten with the value contained in attributes.
//
// If this span is not being recorded than this method does nothing.
func (s *span) SetAttributes(attributes ...attribute.KeyValue) {
	if !s.IsRecording() {
		return
	}
	s.copyToCappedAttributes(attributes...)
}

// End ends the span. This method does nothing if the span is already ended or
// is not being recorded.
//
// The only SpanOption currently supported is WithTimestamp which will set the
// end time for a Span's life-cycle.
//
// If this method is called while panicking an error event is added to the
// Span before ending it and the panic is continued.
func (s *span) End(options ...trace.SpanEndOption) {
	// Do not start by checking if the span is being recorded which requires
	// acquiring a lock. Make a minimal check that the span is not nil.
	if s == nil {
		return
	}

	// Store the end time as soon as possible to avoid artificially increasing
	// the span's duration in case some operation below takes a while.
	et := internal.MonotonicEndTime(s.startTime)

	// Do relative expensive check now that we have an end time and see if we
	// need to do any more processing.
	if !s.IsRecording() {
		return
	}

	if recovered := recover(); recovered != nil {
		// Record but don't stop the panic.
		defer panic(recovered)
		s.addEvent(
			semconv.ExceptionEventName,
			trace.WithAttributes(
				semconv.ExceptionTypeKey.String(typeStr(recovered)),
				semconv.ExceptionMessageKey.String(fmt.Sprint(recovered)),
			),
		)
	}

	if s.executionTracerTaskEnd != nil {
		s.executionTracerTaskEnd()
	}

	config := trace.NewSpanEndConfig(options...)

	s.mu.Lock()
	// Setting endTime to non-zero marks the span as ended and not recording.
	if config.Timestamp().IsZero() {
		s.endTime = et
	} else {
		s.endTime = config.Timestamp()
	}
	s.mu.Unlock()

	sps, ok := s.tracer.provider.spanProcessors.Load().(spanProcessorStates)
	mustExportOrProcess := ok && len(sps) > 0
	if mustExportOrProcess {
		for _, sp := range sps {
			sp.sp.OnEnd(s.snapshot())
		}
	}
}

// RecordError will record err as a span event for this span. An additional call to
// SetStatus is required if the Status of the Span should be set to Error, this method
// does not change the Span status. If this span is not being recorded or err is nil
// than this method does nothing.
func (s *span) RecordError(err error, opts ...trace.EventOption) {
	if s == nil || err == nil || !s.IsRecording() {
		return
	}

	opts = append(opts, trace.WithAttributes(
		semconv.ExceptionTypeKey.String(typeStr(err)),
		semconv.ExceptionMessageKey.String(err.Error()),
	))
	s.addEvent(semconv.ExceptionEventName, opts...)
}

func typeStr(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.PkgPath() == "" && t.Name() == "" {
		// Likely a builtin type.
		return t.String()
	}
	return fmt.Sprintf("%s.%s", t.PkgPath(), t.Name())
}

// AddEvent adds an event with the provided name and options. If this span is
// not being recorded than this method does nothing.
func (s *span) AddEvent(name string, o ...trace.EventOption) {
	if !s.IsRecording() {
		return
	}
	s.addEvent(name, o...)
}

func (s *span) addEvent(name string, o ...trace.EventOption) {
	c := trace.NewEventConfig(o...)

	// Discard over limited attributes
	attributes := c.Attributes()
	var discarded int
	if len(attributes) > s.spanLimits.AttributePerEventCountLimit {
		discarded = len(attributes) - s.spanLimits.AttributePerEventCountLimit
		attributes = attributes[:s.spanLimits.AttributePerEventCountLimit]
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events.add(Event{
		Name:                  name,
		Attributes:            attributes,
		DroppedAttributeCount: discarded,
		Time:                  c.Timestamp(),
	})
}

// SetName sets the name of this span. If this span is not being recorded than
// this method does nothing.
func (s *span) SetName(name string) {
	if !s.IsRecording() {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.name = name
}

// Name returns the name of this span.
func (s *span) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Name returns the SpanContext of this span's parent span.
func (s *span) Parent() trace.SpanContext {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.parent
}

// SpanKind returns the SpanKind of this span.
func (s *span) SpanKind() trace.SpanKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.spanKind
}

// StartTime returns the time this span started.
func (s *span) StartTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTime
}

// EndTime returns the time this span ended. For spans that have not yet
// ended, the returned value will be the zero value of time.Time.
func (s *span) EndTime() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.endTime
}

// Attributes returns the attributes of this span.
func (s *span) Attributes() []attribute.KeyValue {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.attributes.evictList.Len() == 0 {
		return []attribute.KeyValue{}
	}
	return s.attributes.toKeyValue()
}

// Links returns the links of this span.
func (s *span) Links() []trace.Link {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.links.queue) == 0 {
		return []trace.Link{}
	}
	return s.interfaceArrayToLinksArray()
}

// Events returns the events of this span.
func (s *span) Events() []Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.events.queue) == 0 {
		return []Event{}
	}
	return s.interfaceArrayToEventArray()
}

// Status returns the status of this span.
func (s *span) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}

// InstrumentationLibrary returns the instrumentation.Library associated with
// the Tracer that created this span.
func (s *span) InstrumentationLibrary() instrumentation.Library {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.instrumentationLibrary
}

// Resource returns the Resource associated with the Tracer that created this
// span.
func (s *span) Resource() *resource.Resource {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resource
}

func (s *span) addLink(link trace.Link) {
	if !s.IsRecording() {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	// Discard over limited attributes
	if len(link.Attributes) > s.spanLimits.AttributePerLinkCountLimit {
		link.DroppedAttributeCount = len(link.Attributes) - s.spanLimits.AttributePerLinkCountLimit
		link.Attributes = link.Attributes[:s.spanLimits.AttributePerLinkCountLimit]
	}

	s.links.add(link)
}

// DroppedAttributes returns the number of attributes dropped by the span
// due to limits being reached.
func (s *span) DroppedAttributes() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.attributes.droppedCount
}

// DroppedLinks returns the number of links dropped by the span due to limits
// being reached.
func (s *span) DroppedLinks() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.links.droppedCount
}

// DroppedEvents returns the number of events dropped by the span due to
// limits being reached.
func (s *span) DroppedEvents() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.events.droppedCount
}

// ChildSpanCount returns the count of spans that consider the span a
// direct parent.
func (s *span) ChildSpanCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.childSpanCount
}

// TracerProvider returns a trace.TracerProvider that can be used to generate
// additional Spans on the same telemetry pipeline as the current Span.
func (s *span) TracerProvider() trace.TracerProvider {
	return s.tracer.provider
}

// snapshot creates a read-only copy of the current state of the span.
func (s *span) snapshot() ReadOnlySpan {
	var sd snapshot
	s.mu.Lock()
	defer s.mu.Unlock()

	sd.endTime = s.endTime
	sd.instrumentationLibrary = s.instrumentationLibrary
	sd.name = s.name
	sd.parent = s.parent
	sd.resource = s.resource
	sd.spanContext = s.spanContext
	sd.spanKind = s.spanKind
	sd.startTime = s.startTime
	sd.status = s.status
	sd.childSpanCount = s.childSpanCount

	if s.attributes.evictList.Len() > 0 {
		sd.attributes = s.attributes.toKeyValue()
		sd.droppedAttributeCount = s.attributes.droppedCount
	}
	if len(s.events.queue) > 0 {
		sd.events = s.interfaceArrayToEventArray()
		sd.droppedEventCount = s.events.droppedCount
	}
	if len(s.links.queue) > 0 {
		sd.links = s.interfaceArrayToLinksArray()
		sd.droppedLinkCount = s.links.droppedCount
	}
	return &sd
}

func (s *span) interfaceArrayToLinksArray() []trace.Link {
	linkArr := make([]trace.Link, 0)
	for _, value := range s.links.queue {
		linkArr = append(linkArr, value.(trace.Link))
	}
	return linkArr
}

func (s *span) interfaceArrayToEventArray() []Event {
	eventArr := make([]Event, 0)
	for _, value := range s.events.queue {
		eventArr = append(eventArr, value.(Event))
	}
	return eventArr
}

func (s *span) copyToCappedAttributes(attributes ...attribute.KeyValue) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range attributes {
		// Ensure attributes conform to the specification:
		// https://github.com/open-telemetry/opentelemetry-specification/blob/v1.0.1/specification/common/common.md#attributes
		if a.Valid() {
			s.attributes.add(a)
		}
	}
}

func (s *span) addChild() {
	if !s.IsRecording() {
		return
	}
	s.mu.Lock()
	s.childSpanCount++
	s.mu.Unlock()
}

func (*span) private() {}

func startSpanInternal(ctx context.Context, tr *tracer, name string, o *trace.SpanConfig) *span {
	span := &span{}

	provider := tr.provider

	// If told explicitly to make this a new root use a zero value SpanContext
	// as a parent which contains an invalid trace ID and is not remote.
	var psc trace.SpanContext
	if !o.NewRoot() {
		psc = trace.SpanContextFromContext(ctx)
	}

	// If there is a valid parent trace ID, use it to ensure the continuity of
	// the trace. Always generate a new span ID so other components can rely
	// on a unique span ID, even if the Span is non-recording.
	var tid trace.TraceID
	var sid trace.SpanID
	if !psc.TraceID().IsValid() {
		tid, sid = provider.idGenerator.NewIDs(ctx)
	} else {
		tid = psc.TraceID()
		sid = provider.idGenerator.NewSpanID(ctx, tid)
	}

	spanLimits := provider.spanLimits
	span.attributes = newAttributesMap(spanLimits.AttributeCountLimit)
	span.events = newEvictedQueue(spanLimits.EventCountLimit)
	span.links = newEvictedQueue(spanLimits.LinkCountLimit)
	span.spanLimits = spanLimits

	samplingResult := provider.sampler.ShouldSample(SamplingParameters{
		ParentContext: ctx,
		TraceID:       tid,
		Name:          name,
		Kind:          o.SpanKind(),
		Attributes:    o.Attributes(),
		Links:         o.Links(),
	})

	scc := trace.SpanContextConfig{
		TraceID:    tid,
		SpanID:     sid,
		TraceState: samplingResult.Tracestate,
	}
	if isSampled(samplingResult) {
		scc.TraceFlags = psc.TraceFlags() | trace.FlagsSampled
	} else {
		scc.TraceFlags = psc.TraceFlags() &^ trace.FlagsSampled
	}
	span.spanContext = trace.NewSpanContext(scc)

	if !isRecording(samplingResult) {
		return span
	}

	startTime := o.Timestamp()
	if startTime.IsZero() {
		startTime = time.Now()
	}
	span.startTime = startTime

	span.spanKind = trace.ValidateSpanKind(o.SpanKind())
	span.name = name
	span.parent = psc
	span.resource = provider.resource
	span.instrumentationLibrary = tr.instrumentationLibrary

	span.SetAttributes(samplingResult.Attributes...)

	return span
}

func isRecording(s SamplingResult) bool {
	return s.Decision == RecordOnly || s.Decision == RecordAndSample
}

func isSampled(s SamplingResult) bool {
	return s.Decision == RecordAndSample
}

// Status is the classified state of a Span.
type Status struct {
	// Code is an identifier of a Spans state classification.
	Code codes.Code
	// Message is a user hint about why that status was set. It is only
	// applicable when Code is Error.
	Description string
}
