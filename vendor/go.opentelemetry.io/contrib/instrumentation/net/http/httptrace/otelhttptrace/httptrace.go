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

package otelhttptrace

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// Option is a function that allows configuration of the httptrace Extract()
// and Inject() functions
type Option func(*config)

type config struct {
	propagators propagation.TextMapPropagator
}

func newConfig(opts []Option) *config {
	c := &config{propagators: otel.GetTextMapPropagator()}
	for _, o := range opts {
		o(c)
	}
	return c
}

// WithPropagators sets the propagators to use for Extraction and Injection
func WithPropagators(props propagation.TextMapPropagator) Option {
	return func(c *config) {
		c.propagators = props
	}
}

// Extract returns the Attributes, Context Entries, and SpanContext that were encoded by Inject.
func Extract(ctx context.Context, req *http.Request, opts ...Option) ([]attribute.KeyValue, baggage.Baggage, trace.SpanContext) {
	c := newConfig(opts)
	ctx = c.propagators.Extract(ctx, propagation.HeaderCarrier(req.Header))

	attrs := append(
		semconv.HTTPServerAttributesFromHTTPRequest("", "", req),
		semconv.NetAttributesFromHTTPRequest("tcp", req)...,
	)

	return attrs, baggage.FromContext(ctx), trace.SpanContextFromContext(ctx)
}

func Inject(ctx context.Context, req *http.Request, opts ...Option) {
	c := newConfig(opts)
	c.propagators.Inject(ctx, propagation.HeaderCarrier(req.Header))
}
