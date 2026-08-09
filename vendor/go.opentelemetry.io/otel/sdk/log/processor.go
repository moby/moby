// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"

	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/sdk/instrumentation"
)

// Processor handles the processing of log records.
//
// Enabled, OnEmit, and ForceFlush may be called concurrently with themselves
// or each other. It is the responsibility of the Processor to manage this
// concurrency.
//
// A Processor must be registered only once and with a single
// [LoggerProvider]. Registering the same Processor with multiple providers or
// multiple times with the same provider is not supported.
//
// A [LoggerProvider] stops admitting new operations that invoke Enabled,
// OnEmit, or ForceFlush when shutdown starts. Callers that use a Processor
// directly are responsible for coordinating those calls with Shutdown.
type Processor interface {
	// Enabled reports whether the Processor will process for the given context
	// and param.
	//
	// Enabled is called synchronously and should not block.
	//
	// The param contains a subset of the information that will be available
	// in the Record passed to OnEmit, as defined by EnabledParameters.
	// A field being unset in param does not imply the corresponding field
	// in the Record passed to OnEmit will be unset. For example, a log bridge
	// may be unable to populate all fields in EnabledParameters even though
	// they are present on the final Record.
	//
	// The returned value will be true when the Processor will process for the
	// provided context and param, and will be false if the Processor will not
	// process.
	//
	// Implementations that need additional information beyond what is provided
	// in param should treat the decision as indeterminate and default to
	// returning true, unless they have a specific reason to return false
	// (for example, to meet performance or correctness constraints).
	//
	// Processor implementations are expected to re-evaluate the [Record] passed
	// to OnEmit. It is not expected that the caller to OnEmit will
	// use the result from Enabled prior to calling OnEmit.
	//
	// The SDK's Logger.Enabled returns false if all the registered processors
	// return false. Otherwise, it returns true.
	//
	Enabled(ctx context.Context, param EnabledParameters) bool

	// OnEmit is called when a Record is emitted.
	//
	// OnEmit is called synchronously and should not block.
	//
	// OnEmit will be called independent of Enabled. Implementations need to
	// validate the arguments themselves before processing.
	//
	// Implementations should not stop processing a Record solely because the
	// context is canceled.
	//
	// Any retry or recovery logic needed by the Processor must be handled
	// inside this function. The SDK does not implement any retry logic.
	// Errors returned by this function are treated as unrecoverable by the SDK
	// and will be reported to a configured error Handler.
	//
	// The SDK invokes the processors sequentially in the same order as
	// they were registered using WithProcessor.
	// Implementations may synchronously modify the record so that the changes
	// are visible in the next registered processor.
	//
	// Note that Record is not concurrent safe. Therefore, asynchronous
	// processing may cause race conditions. Use Record.Clone
	// to create a copy that shares no state with the original.
	OnEmit(ctx context.Context, record *Record) error

	// Shutdown is called when the SDK shuts down. Any cleanup or release of
	// resources held by the Processor (and any underlying Exporter) should be
	// done in this call.
	//
	// A LoggerProvider calls Shutdown at most once. Before calling it, the
	// LoggerProvider waits for all Enabled, OnEmit, and ForceFlush calls it
	// admitted to complete. If the LoggerProvider's Shutdown context is canceled
	// while waiting, Shutdown is not called.
	//
	// Shutdown must include the effects of ForceFlush.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	//
	// After Shutdown is called, calls to OnEmit, Shutdown, or ForceFlush
	// should perform no operation and return nil error.
	Shutdown(ctx context.Context) error

	// ForceFlush exports log records to the configured Exporter that have not yet
	// been exported.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	ForceFlush(ctx context.Context) error
}

// EnabledParameters represents payload for [Processor]'s Enabled method.
type EnabledParameters struct {
	InstrumentationScope instrumentation.Scope
	Severity             log.Severity
	EventName            string
}
