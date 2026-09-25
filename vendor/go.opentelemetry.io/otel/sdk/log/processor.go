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
	// DO NOT CHANGE: any modification will not be backwards compatible and
	// must never be done outside of a new major release.

	// Enabled reports whether the Processor will process a Record for the given
	// context and param.
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
	// The returned value is true if the Processor would process a Record for the
	// provided context and param, and false otherwise.
	//
	// Implementations that need additional information beyond what is provided
	// in param should treat the decision as indeterminate and default to
	// returning true, unless they have a specific reason to return false
	// (for example, to meet performance or correctness constraints).
	//
	// Processor implementations are expected to re-evaluate the [Record] passed
	// to OnEmit. It is not expected that the caller of OnEmit will
	// use the result of Enabled prior to calling OnEmit.
	//
	// The SDK's Logger.Enabled returns false if all the registered processors
	// return false. Otherwise, it returns true.
	//
	Enabled(ctx context.Context, param EnabledParameters) bool
	// DO NOT CHANGE: any modification will not be backwards compatible and
	// must never be done outside of a new major release.

	// OnEmit is called when a Record is emitted.
	//
	// OnEmit is called synchronously and should not block.
	//
	// OnEmit will be called independently of Enabled. Implementations need to
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
	// The SDK invokes the processors sequentially in the same order as they were
	// registered with WithProcessor.
	// Implementations may synchronously modify the record so that the changes
	// are visible in the next registered processor.
	//
	// Note that Record is not concurrent-safe. Therefore, asynchronous
	// processing may cause race conditions. Use Record.Clone
	// to create a copy that shares no state with the original.
	OnEmit(ctx context.Context, record *Record) error
	// DO NOT CHANGE: any modification will not be backwards compatible and
	// must never be done outside of a new major release.

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
	// should perform no operation and return nil.
	Shutdown(ctx context.Context) error
	// DO NOT CHANGE: any modification will not be backwards compatible and
	// must never be done outside of a new major release.

	// ForceFlush exports any log records that have not yet been exported to the
	// configured Exporter.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	ForceFlush(ctx context.Context) error
	// DO NOT CHANGE: any modification will not be backwards compatible and
	// must never be done outside of a new major release.
}

// EnabledParameters represents the payload for [Processor.Enabled].
type EnabledParameters struct {
	InstrumentationScope instrumentation.Scope
	Severity             log.Severity
	EventName            string
}
