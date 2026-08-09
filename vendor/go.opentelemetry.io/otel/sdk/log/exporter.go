// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel/sdk/log/internal/observ"
)

// Exporter handles the delivery of log records to external receivers.
type Exporter interface {
	// Export transmits log records to a receiver.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	//
	// All retry logic must be contained in this function. The SDK does not
	// implement any retry logic. All errors returned by this function are
	// considered unrecoverable and will be reported to a configured error
	// Handler.
	//
	// Implementations must not retain the records slice.
	//
	// Before modifying a Record, the implementation must use Record.Clone
	// to create a copy that shares no state with the original.
	//
	// Export should never be called concurrently with other Export calls.
	// However, it may be called concurrently with other methods.
	Export(ctx context.Context, records []Record) error

	// Shutdown is called when the SDK shuts down. Any cleanup or release of
	// resources held by the exporter should be done in this call.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	//
	// After Shutdown is called, calls to Export, Shutdown, or ForceFlush
	// should perform no operation and return nil error.
	//
	// Shutdown may be called concurrently with itself or with other methods.
	Shutdown(ctx context.Context) error

	// ForceFlush exports log records to the configured Exporter that have not yet
	// been exported.
	//
	// The deadline or cancellation of the passed context must be honored. An
	// appropriate error should be returned in these situations.
	//
	// ForceFlush may be called concurrently with itself or with other methods.
	ForceFlush(ctx context.Context) error
}

var defaultNoopExporter = &noopExporter{}

type noopExporter struct{}

func (noopExporter) Export(context.Context, []Record) error { return nil }

func (noopExporter) Shutdown(context.Context) error { return nil }

func (noopExporter) ForceFlush(context.Context) error { return nil }

func shutdownExporter(ctx context.Context, exporter Exporter) error {
	err := exporter.ForceFlush(ctx)
	return errors.Join(err, exporter.Shutdown(ctx))
}

// chunkExporter wraps an Exporter's Export method so it is called with
// appropriately sized export payloads. Any payload larger than a defined size
// is chunked into smaller payloads and exported sequentially.
type chunkExporter struct {
	Exporter

	// size is the maximum batch size exported.
	size int
}

// newChunkExporter wraps exporter. Calls to the Export will have their records
// payload chunked so they do not exceed size. If size is less than or equal
// to 0, exporter is returned directly.
func newChunkExporter(exporter Exporter, size int) Exporter {
	if size <= 0 {
		return exporter
	}
	return &chunkExporter{Exporter: exporter, size: size}
}

// Export exports records in chunks no larger than c.size.
func (c chunkExporter) Export(ctx context.Context, records []Record) error {
	n := len(records)
	var errs []error
	for i, j := 0, min(c.size, n); i < n; i, j = i+c.size, min(j+c.size, n) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(append(errs, ctxErr)...)
		}
		if err := c.Exporter.Export(ctx, records[i:j]); err != nil {
			errs = append(errs, err)
		}
		if ctxErr := ctx.Err(); ctxErr != nil {
			return errors.Join(append(errs, ctxErr)...)
		}
	}
	return errors.Join(errs...)
}

// timeoutExporter wraps an Exporter and ensures any call to Export will have a
// timeout for the context.
type timeoutExporter struct {
	Exporter

	// timeout is the maximum time an export is attempted.
	timeout time.Duration
}

// newTimeoutExporter wraps exporter with an Exporter that limits the context
// lifetime passed to Export to be timeout. If timeout is less than or equal to
// zero, exporter will be returned directly.
func newTimeoutExporter(exp Exporter, timeout time.Duration) Exporter {
	if timeout <= 0 {
		return exp
	}
	return &timeoutExporter{Exporter: exp, timeout: timeout}
}

// Export sets the timeout of ctx before calling the Exporter e wraps.
func (e *timeoutExporter) Export(ctx context.Context, records []Record) error {
	// This only used by the batch processor, and it takes processor timeout config.
	// Thus, the error message points to the processor. So users know they should adjust the processor timeout.
	ctx, cancel := context.WithTimeoutCause(ctx, e.timeout, errors.New("processor export timeout"))
	defer cancel()
	return e.Exporter.Export(ctx, records)
}

// metricsExporter wraps an Exporter to record log processing metrics
// just before calling the wrapped exporter.
type metricsExporter struct {
	Exporter
	inst *observ.BLP
}

// newMetricsExporter creates a metricsExporter that wraps the given exporter.
func newMetricsExporter(exporter Exporter, inst *observ.BLP) Exporter {
	return &metricsExporter{
		Exporter: exporter,
		inst:     inst,
	}
}

// Export records the number of log records as a metric then forwards
// them to the wrapped Exporter. Error returned from wrapped exporter
// is not considered as per specification (to be measured by exporter).
func (e *metricsExporter) Export(ctx context.Context, records []Record) error {
	if e.inst != nil {
		e.inst.Processed(ctx, int64(len(records)))
	}
	return e.Exporter.Export(ctx, records)
}
