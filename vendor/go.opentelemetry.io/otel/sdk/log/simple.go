// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/sdk/log/internal/observ"
)

// This is a compile-time check that SimpleProcessor implements Processor.
var _ Processor = (*SimpleProcessor)(nil)

// SimpleProcessor is a processor that synchronously exports log records.
//
// Use [NewSimpleProcessor] to create a SimpleProcessor.
type SimpleProcessor struct {
	mu       sync.Mutex
	exporter Exporter
	inst     *observ.SLP
	noCmp    [0]func() //nolint: unused  // This is indeed used.
}

// NewSimpleProcessor is a simple Processor adapter.
//
// This Processor is not recommended for production use due to its synchronous
// nature, which makes it suitable for testing, debugging, or demonstrating
// other features, but can lead to slow performance and high computational
// overhead. For production environments, use [NewBatchProcessor] instead.
// However, there may be exceptions in which certain
// [Exporter] implementations perform better with this Processor.
func NewSimpleProcessor(exporter Exporter, _ ...SimpleProcessorOption) *SimpleProcessor {
	slp := &SimpleProcessor{
		exporter: exporter,
	}
	var err error
	slp.inst, err = observ.NewSLP(observ.NextSimpleProcessorID())
	if err != nil {
		otel.Handle(err)
	}
	return slp
}

var simpleProcRecordsPool = sync.Pool{
	New: func() any {
		records := make([]Record, 1)
		return &records
	},
}

// Enabled returns true, indicating this Processor will process all records.
func (*SimpleProcessor) Enabled(context.Context, EnabledParameters) bool {
	return true
}

// OnEmit synchronously exports the provided log record.
func (s *SimpleProcessor) OnEmit(ctx context.Context, r *Record) error {
	if s.exporter == nil {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	records := simpleProcRecordsPool.Get().(*[]Record)
	defer func() {
		clear(*records)
		simpleProcRecordsPool.Put(records)
	}()
	(*records)[0] = *r

	if s.inst != nil {
		// Record the log record as processed at the point it is submitted to
		// the exporter, independent of the export outcome.
		s.inst.LogProcessed(ctx)
	}
	return s.exporter.Export(ctx, *records)
}

// Shutdown flushes the exporter before shutting it down.
func (s *SimpleProcessor) Shutdown(ctx context.Context) error {
	if s.exporter == nil {
		return nil
	}

	return shutdownExporter(ctx, s.exporter)
}

// ForceFlush flushes the exporter.
func (s *SimpleProcessor) ForceFlush(ctx context.Context) error {
	if s.exporter == nil {
		return nil
	}

	return s.exporter.ForceFlush(ctx)
}

// SimpleProcessorOption applies a configuration to a [SimpleProcessor].
type SimpleProcessorOption interface {
	apply()
}
