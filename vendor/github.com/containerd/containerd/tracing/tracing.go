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

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	httpconv "go.opentelemetry.io/otel/semconv/v1.17.0/httpconv"
	"go.opentelemetry.io/otel/trace"
)

// StartConfig defines configuration for a new span object.
type StartConfig struct {
	spanOpts []trace.SpanStartOption
}

type SpanOpt func(config *StartConfig)

// WithHTTPRequest marks span as a HTTP request operation from client to server.
// It'll append attributes from the HTTP request object and mark it with `SpanKindClient` type.
func WithHTTPRequest(request *http.Request) SpanOpt {
	return func(config *StartConfig) {
		config.spanOpts = append(config.spanOpts,
			trace.WithSpanKind(trace.SpanKindClient),                 // A client making a request to a server
			trace.WithAttributes(httpconv.ClientRequest(request)...), // Add HTTP attributes
		)
	}
}

// StartSpan starts child span in a context.
func StartSpan(ctx context.Context, opName string, opts ...SpanOpt) (context.Context, *Span) {
	config := StartConfig{}
	for _, fn := range opts {
		fn(&config)
	}
	tracer := otel.Tracer("")
	if parent := trace.SpanFromContext(ctx); parent != nil && parent.SpanContext().IsValid() {
		tracer = parent.TracerProvider().Tracer("")
	}
	ctx, span := tracer.Start(ctx, opName, config.spanOpts...)
	return ctx, &Span{otelSpan: span}
}

// SpanFromContext returns the current Span from the context.
func SpanFromContext(ctx context.Context) *Span {
	return &Span{
		otelSpan: trace.SpanFromContext(ctx),
	}
}

// Span is wrapper around otel trace.Span.
// Span is the individual component of a trace. It represents a
// single named and timed operation of a workflow that is traced.
type Span struct {
	otelSpan trace.Span
}

// End completes the span.
func (s *Span) End() {
	s.otelSpan.End()
}

// AddEvent adds an event with provided name and options.
func (s *Span) AddEvent(name string, options ...trace.EventOption) {
	s.otelSpan.AddEvent(name, options...)
}

// SetStatus sets the status of the current span.
// If an error is encountered, it records the error and sets span status to Error.
func (s *Span) SetStatus(err error) {
	if err != nil {
		s.otelSpan.RecordError(err)
		s.otelSpan.SetStatus(codes.Error, err.Error())
	} else {
		s.otelSpan.SetStatus(codes.Ok, "")
	}
}

// SetAttributes sets kv as attributes of the span.
func (s *Span) SetAttributes(kv ...attribute.KeyValue) {
	s.otelSpan.SetAttributes(kv...)
}

// Name sets the span name by joining a list of strings in dot separated format.
func Name(names ...string) string {
	return makeSpanName(names...)
}

// Attribute takes a key value pair and returns attribute.KeyValue type.
func Attribute(k string, v interface{}) attribute.KeyValue {
	return any(k, v)
}

// HTTPStatusCodeAttributes generates attributes of the HTTP namespace as specified by the OpenTelemetry
// specification for a span.
func HTTPStatusCodeAttributes(code int) []attribute.KeyValue {
	return []attribute.KeyValue{semconv.HTTPStatusCodeKey.Int(code)}
}
