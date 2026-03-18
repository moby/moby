package detect

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sync/semaphore"
)

var Recorder *TraceRecorder

type TraceRecorder struct {
	// sem is a binary semaphore for this struct.
	// This is used instead of sync.Mutex because it allows
	// for context cancellation to work properly.
	sem *semaphore.Weighted

	// shutdown function for the gc.
	shutdownGC func(err error)

	// done channel that marks when background goroutines
	// are closed.
	done chan struct{}

	// track traces and listeners for traces.
	m         map[trace.TraceID]*stubs
	listeners map[trace.TraceID]int
}

type stubs struct {
	spans []tracetest.SpanStub
	last  time.Time
}

func NewTraceRecorder() *TraceRecorder {
	tr := &TraceRecorder{
		sem:       semaphore.NewWeighted(1),
		done:      make(chan struct{}),
		m:         map[trace.TraceID]*stubs{},
		listeners: map[trace.TraceID]int{},
	}

	ctx, cancel := context.WithCancelCause(context.Background())
	go tr.gcLoop(ctx)
	tr.shutdownGC = cancel

	return tr
}

// Record signals to the TraceRecorder that it should track spans associated with the current
// trace and returns a function that will return these spans.
//
// If the TraceRecorder is nil or there is no valid active span, the returned function
// will be nil to signal that the trace cannot be recorded.
func (r *TraceRecorder) Record(ctx context.Context) (func() []tracetest.SpanStub, error) {
	if r == nil {
		return nil, nil
	}

	spanCtx := trace.SpanContextFromContext(ctx)
	if !spanCtx.IsValid() {
		return nil, nil
	}

	if err := r.sem.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	defer r.sem.Release(1)

	traceID := spanCtx.TraceID()
	r.listeners[traceID]++

	var (
		once  sync.Once
		spans []tracetest.SpanStub
	)
	return func() []tracetest.SpanStub {
		once.Do(func() {
			if err := r.sem.Acquire(context.Background(), 1); err != nil {
				return
			}
			defer r.sem.Release(1)

			if v, ok := r.m[traceID]; ok {
				spans = v.spans
			}
			r.listeners[traceID]--
			if r.listeners[traceID] == 0 {
				delete(r.listeners, traceID)
			}
		})
		return spans
	}, nil
}

func (r *TraceRecorder) gcLoop(ctx context.Context) {
	defer close(r.done)

	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.gc(ctx, now)
		}
	}
}

func (r *TraceRecorder) gc(ctx context.Context, now time.Time) {
	if err := r.sem.Acquire(ctx, 1); err != nil {
		return
	}
	defer r.sem.Release(1)

	for k, s := range r.m {
		if _, ok := r.listeners[k]; ok {
			continue
		}
		if now.Sub(s.last) > time.Minute {
			delete(r.m, k)
		}
	}
}

func (r *TraceRecorder) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	if err := r.sem.Acquire(ctx, 1); err != nil {
		return err
	}
	defer r.sem.Release(1)

	now := time.Now()
	for _, s := range spans {
		ss := tracetest.SpanStubFromReadOnlySpan(s)
		v, ok := r.m[ss.SpanContext.TraceID()]
		if !ok {
			v = &stubs{}
			r.m[s.SpanContext().TraceID()] = v
		}
		v.last = now
		v.spans = append(v.spans, ss)
	}
	return nil
}

func (r *TraceRecorder) Shutdown(ctx context.Context) error {
	// Initiate the shutdown of the gc loop.
	r.shutdownGC(errors.WithStack(context.Canceled))

	// Wait for it to be done or the context is canceled.
	select {
	case <-r.done:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}
