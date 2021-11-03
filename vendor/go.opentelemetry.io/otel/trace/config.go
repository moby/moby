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

package trace // import "go.opentelemetry.io/otel/trace"

import (
	"time"

	"go.opentelemetry.io/otel/attribute"
)

// TracerConfig is a group of options for a Tracer.
type TracerConfig struct {
	instrumentationVersion string
	// Schema URL of the telemetry emitted by the Tracer.
	schemaURL string
}

// InstrumentationVersion returns the version of the library providing instrumentation.
func (t *TracerConfig) InstrumentationVersion() string {
	return t.instrumentationVersion
}

// SchemaURL returns the Schema URL of the telemetry emitted by the Tracer.
func (t *TracerConfig) SchemaURL() string {
	return t.schemaURL
}

// NewTracerConfig applies all the options to a returned TracerConfig.
func NewTracerConfig(options ...TracerOption) *TracerConfig {
	config := new(TracerConfig)
	for _, option := range options {
		option.apply(config)
	}
	return config
}

// TracerOption applies an option to a TracerConfig.
type TracerOption interface {
	apply(*TracerConfig)
}

type tracerOptionFunc func(*TracerConfig)

func (fn tracerOptionFunc) apply(cfg *TracerConfig) {
	fn(cfg)
}

// SpanConfig is a group of options for a Span.
type SpanConfig struct {
	attributes []attribute.KeyValue
	timestamp  time.Time
	links      []Link
	newRoot    bool
	spanKind   SpanKind
}

// Attributes describe the associated qualities of a Span.
func (cfg *SpanConfig) Attributes() []attribute.KeyValue {
	return cfg.attributes
}

// Timestamp is a time in a Span life-cycle.
func (cfg *SpanConfig) Timestamp() time.Time {
	return cfg.timestamp
}

// Links are the associations a Span has with other Spans.
func (cfg *SpanConfig) Links() []Link {
	return cfg.links
}

// NewRoot identifies a Span as the root Span for a new trace. This is
// commonly used when an existing trace crosses trust boundaries and the
// remote parent span context should be ignored for security.
func (cfg *SpanConfig) NewRoot() bool {
	return cfg.newRoot
}

// SpanKind is the role a Span has in a trace.
func (cfg *SpanConfig) SpanKind() SpanKind {
	return cfg.spanKind
}

// NewSpanStartConfig applies all the options to a returned SpanConfig.
// No validation is performed on the returned SpanConfig (e.g. no uniqueness
// checking or bounding of data), it is left to the SDK to perform this
// action.
func NewSpanStartConfig(options ...SpanStartOption) *SpanConfig {
	c := new(SpanConfig)
	for _, option := range options {
		option.applySpanStart(c)
	}
	return c
}

// NewSpanEndConfig applies all the options to a returned SpanConfig.
// No validation is performed on the returned SpanConfig (e.g. no uniqueness
// checking or bounding of data), it is left to the SDK to perform this
// action.
func NewSpanEndConfig(options ...SpanEndOption) *SpanConfig {
	c := new(SpanConfig)
	for _, option := range options {
		option.applySpanEnd(c)
	}
	return c
}

// SpanStartOption applies an option to a SpanConfig. These options are applicable
// only when the span is created
type SpanStartOption interface {
	applySpanStart(*SpanConfig)
}

type spanOptionFunc func(*SpanConfig)

func (fn spanOptionFunc) applySpanStart(cfg *SpanConfig) {
	fn(cfg)
}

// SpanEndOption applies an option to a SpanConfig. These options are
// applicable only when the span is ended.
type SpanEndOption interface {
	applySpanEnd(*SpanConfig)
}

// EventConfig is a group of options for an Event.
type EventConfig struct {
	attributes []attribute.KeyValue
	timestamp  time.Time
}

// Attributes describe the associated qualities of an Event.
func (cfg *EventConfig) Attributes() []attribute.KeyValue {
	return cfg.attributes
}

// Timestamp is a time in an Event life-cycle.
func (cfg *EventConfig) Timestamp() time.Time {
	return cfg.timestamp
}

// NewEventConfig applies all the EventOptions to a returned SpanConfig. If no
// timestamp option is passed, the returned SpanConfig will have a Timestamp
// set to the call time, otherwise no validation is performed on the returned
// SpanConfig.
func NewEventConfig(options ...EventOption) *EventConfig {
	c := new(EventConfig)
	for _, option := range options {
		option.applyEvent(c)
	}
	if c.timestamp.IsZero() {
		c.timestamp = time.Now()
	}
	return c
}

// EventOption applies span event options to an EventConfig.
type EventOption interface {
	applyEvent(*EventConfig)
}

// SpanOption are options that can be used at both the beginning and end of a span.
type SpanOption interface {
	SpanStartOption
	SpanEndOption
}

// SpanStartEventOption are options that can be used at the start of a span, or with an event.
type SpanStartEventOption interface {
	SpanStartOption
	EventOption
}

type attributeOption []attribute.KeyValue

func (o attributeOption) applySpan(c *SpanConfig) {
	c.attributes = append(c.attributes, []attribute.KeyValue(o)...)
}
func (o attributeOption) applySpanStart(c *SpanConfig) { o.applySpan(c) }
func (o attributeOption) applyEvent(c *EventConfig) {
	c.attributes = append(c.attributes, []attribute.KeyValue(o)...)
}

var _ SpanStartEventOption = attributeOption{}

// WithAttributes adds the attributes related to a span life-cycle event.
// These attributes are used to describe the work a Span represents when this
// option is provided to a Span's start or end events. Otherwise, these
// attributes provide additional information about the event being recorded
// (e.g. error, state change, processing progress, system event).
//
// If multiple of these options are passed the attributes of each successive
// option will extend the attributes instead of overwriting. There is no
// guarantee of uniqueness in the resulting attributes.
func WithAttributes(attributes ...attribute.KeyValue) SpanStartEventOption {
	return attributeOption(attributes)
}

// SpanEventOption are options that can be used with an event or a span.
type SpanEventOption interface {
	SpanOption
	EventOption
}

type timestampOption time.Time

func (o timestampOption) applySpan(c *SpanConfig)      { c.timestamp = time.Time(o) }
func (o timestampOption) applySpanStart(c *SpanConfig) { o.applySpan(c) }
func (o timestampOption) applySpanEnd(c *SpanConfig)   { o.applySpan(c) }
func (o timestampOption) applyEvent(c *EventConfig)    { c.timestamp = time.Time(o) }

var _ SpanEventOption = timestampOption{}

// WithTimestamp sets the time of a Span or Event life-cycle moment (e.g.
// started, stopped, errored).
func WithTimestamp(t time.Time) SpanEventOption {
	return timestampOption(t)
}

// WithLinks adds links to a Span. The links are added to the existing Span
// links, i.e. this does not overwrite.
func WithLinks(links ...Link) SpanStartOption {
	return spanOptionFunc(func(cfg *SpanConfig) {
		cfg.links = append(cfg.links, links...)
	})
}

// WithNewRoot specifies that the Span should be treated as a root Span. Any
// existing parent span context will be ignored when defining the Span's trace
// identifiers.
func WithNewRoot() SpanStartOption {
	return spanOptionFunc(func(cfg *SpanConfig) {
		cfg.newRoot = true
	})
}

// WithSpanKind sets the SpanKind of a Span.
func WithSpanKind(kind SpanKind) SpanStartOption {
	return spanOptionFunc(func(cfg *SpanConfig) {
		cfg.spanKind = kind
	})
}

// WithInstrumentationVersion sets the instrumentation version.
func WithInstrumentationVersion(version string) TracerOption {
	return tracerOptionFunc(func(cfg *TracerConfig) {
		cfg.instrumentationVersion = version
	})
}

// WithSchemaURL sets the schema URL for the Tracer.
func WithSchemaURL(schemaURL string) TracerOption {
	return tracerOptionFunc(func(cfg *TracerConfig) {
		cfg.schemaURL = schemaURL
	})
}
