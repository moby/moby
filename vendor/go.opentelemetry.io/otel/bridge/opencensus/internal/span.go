// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal // import "go.opentelemetry.io/otel/bridge/opencensus/internal"

import (
	"fmt"

	octrace "go.opencensus.io/trace"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/bridge/opencensus/internal/oc2otel"
	"go.opentelemetry.io/otel/bridge/opencensus/internal/otel2oc"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	// MessageSendEvent is the name of the message send event.
	MessageSendEvent = "message send"
	// MessageReceiveEvent is the name of the message receive event.
	MessageReceiveEvent = "message receive"
)

var (
	// UncompressedKey is used for the uncompressed byte size attribute.
	UncompressedKey = attribute.Key("uncompressed byte size")
	// CompressedKey is used for the compressed byte size attribute.
	CompressedKey = attribute.Key("compressed byte size")
)

// Span is an OpenCensus SpanInterface wrapper for an OpenTelemetry Span.
type Span struct {
	otelSpan trace.Span
}

// NewSpan returns an OpenCensus Span wrapping an OpenTelemetry Span.
func NewSpan(s trace.Span) *octrace.Span {
	return octrace.NewSpan(&Span{otelSpan: s})
}

// IsRecordingEvents reports whether events are being recorded for this span.
func (s *Span) IsRecordingEvents() bool {
	return s.otelSpan.IsRecording()
}

// End ends this span.
func (s *Span) End() {
	s.otelSpan.End()
}

// SpanContext returns the SpanContext of this span.
func (s *Span) SpanContext() octrace.SpanContext {
	return otel2oc.SpanContext(s.otelSpan.SpanContext())
}

// SetName sets the name of this span, if it is recording events.
func (s *Span) SetName(name string) {
	s.otelSpan.SetName(name)
}

// SetStatus sets the status of this span, if it is recording events.
func (s *Span) SetStatus(status octrace.Status) {
	s.otelSpan.SetStatus(codes.Code(max(0, status.Code)), status.Message) // nolint:gosec // Overflow checked.
}

// AddAttributes sets attributes in this span.
func (s *Span) AddAttributes(attributes ...octrace.Attribute) {
	s.otelSpan.SetAttributes(oc2otel.Attributes(attributes)...)
}

// Annotate adds an annotation with attributes to this span.
func (s *Span) Annotate(attributes []octrace.Attribute, str string) {
	s.otelSpan.AddEvent(str, trace.WithAttributes(oc2otel.Attributes(attributes)...))
}

// Annotatef adds a formatted annotation with attributes to this span.
func (s *Span) Annotatef(attributes []octrace.Attribute, format string, a ...any) {
	s.Annotate(attributes, fmt.Sprintf(format, a...))
}

// AddMessageSendEvent adds a message send event to this span.
func (s *Span) AddMessageSendEvent(_, uncompressedByteSize, compressedByteSize int64) {
	s.otelSpan.AddEvent(MessageSendEvent,
		trace.WithAttributes(
			attribute.KeyValue{
				Key:   UncompressedKey,
				Value: attribute.Int64Value(uncompressedByteSize),
			},
			attribute.KeyValue{
				Key:   CompressedKey,
				Value: attribute.Int64Value(compressedByteSize),
			}),
	)
}

// AddMessageReceiveEvent adds a message receive event to this span.
func (s *Span) AddMessageReceiveEvent(_, uncompressedByteSize, compressedByteSize int64) {
	s.otelSpan.AddEvent(MessageReceiveEvent,
		trace.WithAttributes(
			attribute.KeyValue{
				Key:   UncompressedKey,
				Value: attribute.Int64Value(uncompressedByteSize),
			},
			attribute.KeyValue{
				Key:   CompressedKey,
				Value: attribute.Int64Value(compressedByteSize),
			}),
	)
}

// AddLink adds a link to this span.
// This drops the OpenCensus LinkType because there is no such concept in OpenTelemetry.
func (s *Span) AddLink(l octrace.Link) {
	s.otelSpan.AddLink(trace.Link{
		SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
			TraceID: trace.TraceID(l.TraceID),
			SpanID:  trace.SpanID(l.SpanID),
			// We don't know if this was sampled or not.
			// Mark it as sampled, since sampled means
			// "the caller may have recorded trace data":
			// https://www.w3.org/TR/trace-context/#sampled-flag
			TraceFlags: trace.FlagsSampled,
		}),
		Attributes: oc2otel.AttributesFromMap(l.Attributes),
	})
}

// String prints a string representation of this span.
func (s *Span) String() string {
	return "span " + s.otelSpan.SpanContext().SpanID().String()
}
