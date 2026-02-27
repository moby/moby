// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package baggagecopy // import "go.opentelemetry.io/contrib/processors/baggagecopy"

import (
	"context"

	"go.opentelemetry.io/otel/baggage"
	api "go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/log"
)

// LogProcessor is a [log.Processor] implementation that adds baggage
// members onto a log as attributes.
type LogProcessor struct {
	filter Filter
}

var _ log.Processor = (*LogProcessor)(nil)

// NewLogProcessor returns a new [LogProcessor].
//
// The Baggage log processor adds attributes to a log record that are found
// in Baggage in the parent context at the moment the log is emitted.
// The passed filter determines which baggage members are added to the span.
//
// If filter is nil, all baggage members will be added.
func NewLogProcessor(filter Filter) *LogProcessor {
	return &LogProcessor{
		filter: filter,
	}
}

// Enabled reports whether the Processor will process.
func (LogProcessor) Enabled(context.Context, log.EnabledParameters) bool {
	return true
}

// OnEmit adds Baggage member to a log record as attributes that are pulled from
// the Baggage found in ctx. Baggage members are filtered by the filter passed
// to NewLogProcessor.
func (processor LogProcessor) OnEmit(ctx context.Context, record *log.Record) error {
	filter := processor.filter
	if filter == nil {
		filter = AllowAllMembers
	}

	for _, member := range baggage.FromContext(ctx).Members() {
		if filter(member) {
			record.AddAttributes(api.String(member.Key(), member.Value()))
		}
	}

	return nil
}

// Shutdown is called when the [log.Processor] is shutting down and is a no-op for this processor.
func (LogProcessor) Shutdown(context.Context) error { return nil }

// ForceFlush is called to ensure all logs are flushed to the output and is a no-op for this processor.
func (LogProcessor) ForceFlush(context.Context) error { return nil }
