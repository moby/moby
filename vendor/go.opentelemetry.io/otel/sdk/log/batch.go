// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"context"
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/internal/global"
	"go.opentelemetry.io/otel/sdk/log/internal/counter"
	"go.opentelemetry.io/otel/sdk/log/internal/observ"
)

const (
	dfltMaxQSize        = 2048
	dfltExpInterval     = time.Second
	dfltExpTimeout      = 30 * time.Second
	dfltExpMaxBatchSize = 512

	envarMaxQSize        = "OTEL_BLRP_MAX_QUEUE_SIZE"
	envarExpInterval     = "OTEL_BLRP_SCHEDULE_DELAY"
	envarExpTimeout      = "OTEL_BLRP_EXPORT_TIMEOUT"
	envarExpMaxBatchSize = "OTEL_BLRP_MAX_EXPORT_BATCH_SIZE"
)

// Compile-time check BatchProcessor implements Processor.
var _ Processor = (*BatchProcessor)(nil)

// BatchProcessor is a processor that exports batches of log records.
//
// Use [NewBatchProcessor] to create a BatchProcessor. An empty BatchProcessor
// is shut down by default, no records will be batched or exported.
type BatchProcessor struct {
	// A single goroutine owns dequeueing and all exporter calls. OnEmit only
	// writes to the bounded queue and signals that goroutine. Consequently,
	// exporter backpressure blocks the exporter goroutine instead of causing
	// another goroutine to retry without making progress.
	exporter Exporter

	// q is the active queue of records that have not yet been exported.
	q *queue
	// batchSize is the maximum number of records in a scheduled export.
	batchSize int

	// exportTrigger is a coalesced signal that records are ready to export.
	exportTrigger chan struct{}
	// flush serializes ForceFlush requests through the worker.
	flush chan batchProcessorRequest
	// shutdown accepts the single Shutdown request. It is separate from flush
	// so shutdown cannot be blocked behind concurrent ForceFlush callers.
	shutdown chan batchProcessorRequest
	// done is closed by the exporter goroutine after exporter shutdown.
	done chan struct{}

	// stopped holds the stopped state of the BatchProcessor.
	stopped atomic.Bool

	// inst is the instrumentation for observability (nil when disabled).
	inst *observ.BLP

	noCmp [0]func() //nolint: unused  // This is indeed used.
}

type batchProcessorRequest struct {
	ctx  context.Context
	resp chan<- error
}

func (r batchProcessorRequest) respond(err error) {
	r.resp <- err
}

// NewBatchProcessor decorates the provided exporter
// so that the log records are batched before exporting.
//
// Calls to the exporter's Export, ForceFlush, and Shutdown methods are
// synchronized and never invoked concurrently.
func NewBatchProcessor(exporter Exporter, opts ...BatchProcessorOption) *BatchProcessor {
	cfg := newBatchConfig(opts)
	if exporter == nil {
		// Do not panic on nil export.
		exporter = defaultNoopExporter
	}

	b := &BatchProcessor{
		q:             newQueue(cfg.maxQSize.Value),
		batchSize:     cfg.expMaxBatchSize.Value,
		exportTrigger: make(chan struct{}, 1),
		flush:         make(chan batchProcessorRequest),
		shutdown:      make(chan batchProcessorRequest, 1),
		done:          make(chan struct{}),
	}

	var err error
	b.inst, err = observ.NewBLP(
		counter.NextExporterID(),
		func() int64 { return int64(b.q.Len()) },
		int64(cfg.maxQSize.Value),
	)
	if err != nil {
		otel.Handle(err)
	}

	// Wrap exporter with metrics recording if observability is enabled.
	// This must be the innermost wrapper (closest to user exporter) to record
	// metrics just before calling the actual exporter.
	if b.inst != nil {
		exporter = newMetricsExporter(exporter, b.inst)
	}

	// Order is important here. Wrap the timeoutExporter with the chunkExporter
	// to ensure each export completes in timeout (instead of all chunked
	// exports).
	exporter = newTimeoutExporter(exporter, cfg.expTimeout.Value)
	// Use a chunkExporter to ensure ForceFlush and Shutdown calls are batched
	// appropriately on export.
	exporter = newChunkExporter(exporter, cfg.expMaxBatchSize.Value)

	b.exporter = exporter
	b.process(cfg.expInterval.Value)
	return b
}

// process starts the goroutine that owns dequeueing and all exporter calls.
func (b *BatchProcessor) process(interval time.Duration) {
	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()
		// The worker owns and reuses buf. Exporters must not retain the slice
		// passed to them, so it is safe to refill after Export returns.
		buf := make([]Record, b.batchSize)

		for {
			// Probe shutdown by itself first. This makes an already queued terminal
			// request win over every other ready case. Closing done before replying
			// also means a successful Shutdown response observes a stopped worker.
			select {
			case req := <-b.shutdown:
				err := b.shutdownExporter(req.ctx)
				close(b.done)
				req.respond(err)
				return
			default:
			}

			// With no queued shutdown, service a waiting ForceFlush before ordinary
			// export wakes. The default keeps this priority check non-blocking.
			// Shutdown remains selectable in case it arrived after the first probe.
			select {
			case req := <-b.shutdown:
				err := b.shutdownExporter(req.ctx)
				close(b.done)
				req.respond(err)
				return
			case req := <-b.flush:
				err := b.flushExporter(req.ctx)
				req.respond(err)
				continue
			default:
			}

			// No lifecycle request was waiting, so block on the complete event set.
			// Both timer and size-triggered exports start a new interval window.
			select {
			case req := <-b.shutdown:
				err := b.shutdownExporter(req.ctx)
				close(b.done)
				req.respond(err)
				return
			case req := <-b.flush:
				err := b.flushExporter(req.ctx)
				req.respond(err)
			case <-timer.C:
				resetTimer(timer, interval)
				b.exportBatch(buf)
			case <-b.exportTrigger:
				resetTimer(timer, interval)
				b.exportBatch(buf)
			}
		}
	}()
}

func resetTimer(timer *time.Timer, interval time.Duration) {
	// Handle both GODEBUG=asynctimerchan=[0|1] properly.
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(interval)
}

func (b *BatchProcessor) exportBatch(buf []Record) {
	b.logDroppedRecords()
	n, remaining := b.q.Dequeue(buf)
	if n == 0 {
		return
	}

	err := b.exporter.Export(context.Background(), buf[:n])
	clear(buf[:n])
	if err != nil {
		otel.Handle(err)
	}
	if remaining >= b.batchSize {
		b.triggerExport()
	}
}

func (b *BatchProcessor) flushExporter(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.logDroppedRecords()
	records := b.q.Flush()
	err := b.exporter.Export(ctx, records)
	clear(records)
	if ctxErr := ctx.Err(); ctxErr != nil {
		return errors.Join(err, ctxErr)
	}
	return errors.Join(err, b.exporter.ForceFlush(ctx))
}

func (b *BatchProcessor) shutdownExporter(ctx context.Context) error {
	b.logDroppedRecords()
	records := b.q.Flush()
	err := b.exporter.Export(ctx, records)
	clear(records)
	if ctxErr := ctx.Err(); ctxErr != nil {
		err = errors.Join(err, ctxErr)
	} else {
		err = errors.Join(err, b.exporter.ForceFlush(ctx))
	}
	err = errors.Join(err, b.exporter.Shutdown(ctx))
	if b.inst != nil {
		err = errors.Join(err, b.inst.Shutdown())
	}
	return err
}

func (b *BatchProcessor) logDroppedRecords() {
	if d := b.q.Dropped(); d > 0 {
		if b.inst != nil {
			b.inst.ProcessedQueueFull(context.Background(), int64(min(math.MaxInt64, d))) // nolint:gosec
		}
		global.Warn("dropped log records", "dropped", d)
	}
}

func (b *BatchProcessor) triggerExport() {
	select {
	case b.exportTrigger <- struct{}{}:
	default:
	}
}

// Enabled returns true, indicating this Processor will process all records.
func (*BatchProcessor) Enabled(context.Context, EnabledParameters) bool {
	return true
}

// OnEmit batches provided log record.
func (b *BatchProcessor) OnEmit(_ context.Context, r *Record) error {
	if b.stopped.Load() || b.q == nil {
		return nil
	}
	// The record is cloned so that changes done by subsequent processors
	// are not going to lead to a data race.
	if n, accepted := b.q.Enqueue(r.Clone()); accepted && n >= b.batchSize {
		b.triggerExport()
	}
	return nil
}

// Shutdown flushes queued log records and the decorated exporter before
// shutting it down.
func (b *BatchProcessor) Shutdown(ctx context.Context) error {
	if b.stopped.Swap(true) || b.q == nil {
		return nil
	}

	b.q.Close()
	resp := make(chan error, 1)
	b.shutdown <- batchProcessorRequest{ctx: ctx, resp: resp}
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ForceFlush flushes queued log records and flushes the decorated exporter.
func (b *BatchProcessor) ForceFlush(ctx context.Context) error {
	if b.stopped.Load() || b.q == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	resp := make(chan error, 1)
	req := batchProcessorRequest{ctx: ctx, resp: resp}
	select {
	case b.flush <- req:
	case <-b.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}

	select {
	case err := <-resp:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// queue holds a queue of logging records.
//
// When the queue becomes full, the oldest records in the queue are
// overwritten.
type queue struct {
	sync.Mutex

	dropped     atomic.Uint64
	cap, len    int
	read, write *ring
	closed      bool
}

func newQueue(size int) *queue {
	r := newRing(size)
	return &queue{
		cap:   size,
		read:  r,
		write: r,
	}
}

func (q *queue) Len() int {
	q.Lock()
	defer q.Unlock()

	return q.len
}

// Dropped returns the number of Records dropped during enqueueing since the
// last time Dropped was called.
func (q *queue) Dropped() uint64 {
	return q.dropped.Swap(0)
}

// Enqueue adds r to the queue. The queue size, including the addition of r, is
// returned.
//
// If enqueueing r will exceed the capacity of q, the oldest Record held in q
// will be dropped and r retained.
func (q *queue) Enqueue(r Record) (int, bool) {
	q.Lock()
	defer q.Unlock()

	if q.closed {
		return q.len, false
	}

	q.write.Value = r
	q.write = q.write.Next()

	q.len++
	if q.len > q.cap {
		// Overflow. Advance read to be the new "oldest".
		q.len = q.cap
		q.read = q.read.Next()
		q.dropped.Add(1)
	}
	return q.len, true
}

// Dequeue removes up to len(buf) records from the queue and copies them into
// buf. The number copied and the number remaining are returned.
func (q *queue) Dequeue(buf []Record) (int, int) {
	q.Lock()
	defer q.Unlock()

	n := min(len(buf), q.len)
	for i := range n {
		buf[i] = q.read.Value // nolint:gosec // n is bounded by len(buf)
		q.read.Value = Record{}
		q.read = q.read.Next()
	}
	q.len -= n
	return n, q.len
}

// Flush returns all the Records held in the queue and resets it to be
// empty.
func (q *queue) Flush() []Record {
	q.Lock()
	defer q.Unlock()

	return q.flush()
}

// Close stops the queue from accepting records.
func (q *queue) Close() {
	q.Lock()
	defer q.Unlock()

	q.closed = true
}

func (q *queue) flush() []Record {
	out := make([]Record, q.len)
	for i := range out {
		out[i] = q.read.Value
		q.read.Value = Record{}
		q.read = q.read.Next()
	}
	q.len = 0

	return out
}

type batchConfig struct {
	maxQSize        setting[int]
	expInterval     setting[time.Duration]
	expTimeout      setting[time.Duration]
	expMaxBatchSize setting[int]
}

func newBatchConfig(options []BatchProcessorOption) batchConfig {
	var c batchConfig
	for _, o := range options {
		c = o.apply(c)
	}

	c.maxQSize = c.maxQSize.Resolve(
		clearLessThanOne[int](),
		getenv[int](envarMaxQSize),
		clearLessThanOne[int](), // nolint:gocritic // the function argument is duplicated on purpose
		fallback[int](dfltMaxQSize),
	)
	c.expInterval = c.expInterval.Resolve(
		clearLessThanOne[time.Duration](),
		getenv[time.Duration](envarExpInterval),
		clearLessThanOne[time.Duration](), // nolint:gocritic // the function argument is duplicated on purpose
		fallback[time.Duration](dfltExpInterval),
	)
	c.expTimeout = c.expTimeout.Resolve(
		clearLessThanOne[time.Duration](),
		getenv[time.Duration](envarExpTimeout),
		clearLessThanOne[time.Duration](), // nolint:gocritic // the function argument is duplicated on purpose
		fallback[time.Duration](dfltExpTimeout),
	)
	c.expMaxBatchSize = c.expMaxBatchSize.Resolve(
		clearLessThanOne[int](),
		getenv[int](envarExpMaxBatchSize),
		clearLessThanOne[int](), // nolint:gocritic // the function argument is duplicated on purpose
		fallback[int](dfltExpMaxBatchSize),
		clampMax[int](c.maxQSize.Value),
	)
	return c
}

// BatchProcessorOption applies a configuration to a [BatchProcessor].
type BatchProcessorOption interface {
	apply(batchConfig) batchConfig
}

type batchOptionFunc func(batchConfig) batchConfig

func (fn batchOptionFunc) apply(c batchConfig) batchConfig {
	return fn(c)
}

// WithMaxQueueSize sets the maximum queue size used by the Batcher.
// After the size is reached log records are dropped.
//
// If the OTEL_BLRP_MAX_QUEUE_SIZE environment variable is set,
// and this option is not passed, that variable value will be used.
//
// By default, if an environment variable is not set, and this option is not
// passed, 2048 will be used.
// The default value is also used when the provided value is less than one.
func WithMaxQueueSize(size int) BatchProcessorOption {
	return batchOptionFunc(func(cfg batchConfig) batchConfig {
		cfg.maxQSize = newSetting(size)
		return cfg
	})
}

// WithExportInterval sets the maximum duration between batched exports.
//
// If the OTEL_BLRP_SCHEDULE_DELAY environment variable is set,
// and this option is not passed, that variable value will be used.
//
// By default, if an environment variable is not set, and this option is not
// passed, 1s will be used.
// The default value is also used when the provided value is less than one.
func WithExportInterval(d time.Duration) BatchProcessorOption {
	return batchOptionFunc(func(cfg batchConfig) batchConfig {
		cfg.expInterval = newSetting(d)
		return cfg
	})
}

// WithExportTimeout sets the duration after which a batched export is canceled.
//
// If the OTEL_BLRP_EXPORT_TIMEOUT environment variable is set,
// and this option is not passed, that variable value will be used.
//
// By default, if an environment variable is not set, and this option is not
// passed, 30s will be used.
// The default value is also used when the provided value is less than one.
func WithExportTimeout(d time.Duration) BatchProcessorOption {
	return batchOptionFunc(func(cfg batchConfig) batchConfig {
		cfg.expTimeout = newSetting(d)
		return cfg
	})
}

// WithExportMaxBatchSize sets the maximum batch size of every export.
// A batch will be split into multiple exports to not exceed this size.
//
// If the OTEL_BLRP_MAX_EXPORT_BATCH_SIZE environment variable is set,
// and this option is not passed, that variable value will be used.
//
// By default, if an environment variable is not set, and this option is not
// passed, 512 or the maximum queue size, if smaller, will be used.
// The default value is also used when the provided value is less than one.
// The effective batch size will not exceed the configured maximum queue size.
func WithExportMaxBatchSize(size int) BatchProcessorOption {
	return batchOptionFunc(func(cfg batchConfig) batchConfig {
		cfg.expMaxBatchSize = newSetting(size)
		return cfg
	})
}

// WithExportBufferSize is retained for source compatibility and has no effect.
// The processor no longer maintains a separately configurable export-request
// buffer. [WithMaxQueueSize] bounds the pending-record queue.
//
// Deprecated: This option is no longer used.
func WithExportBufferSize(_ int) BatchProcessorOption {
	return batchOptionFunc(func(cfg batchConfig) batchConfig {
		return cfg
	})
}
