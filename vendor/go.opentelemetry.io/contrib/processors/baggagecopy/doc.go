// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package baggagecopy is an OpenTelemetry [Span Processor] that reads key/values
// stored in [Baggage] in the starting span's parent context and adds them as
// attributes to the span.
//
// Keys and values added to Baggage will appear on all subsequent child spans for
// a trace within this service and will be propagated to external services via
// propagation headers.
// If the external services also have a Baggage span processor, the keys and
// values will appear in those child spans as well.
//
// Do not put sensitive information in Baggage.
//
// # Usage
//
// Add the span processor when configuring the tracer provider.
//
// The convenience function [AllowAllBaggageKeys] is provided to
// allow all baggage keys to be copied to the span. Alternatively, you can
// provide a custom baggage key predicate to select which baggage keys you want
// to copy.
//
// [Span Processor]: https://opentelemetry.io/docs/specs/otel/trace/sdk/#span-processor
// [Baggage]: https://opentelemetry.io/docs/specs/otel/api/baggage
package baggagecopy // import "go.opentelemetry.io/contrib/processors/baggagecopy"
