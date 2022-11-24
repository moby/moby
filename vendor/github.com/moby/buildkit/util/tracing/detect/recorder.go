package detect

import (
	"context"
	"sync"
	"time"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

type TraceRecorder struct {
	sdktrace.SpanExporter

	mu        sync.Mutex
	m         map[trace.TraceID]*stubs
	listeners map[trace.TraceID]int
	flush     func(context.Context) error
}

type stubs struct {
	spans []tracetest.SpanStub
	last  time.Time
}

func NewTraceRecorder() *TraceRecorder {
	tr := &TraceRecorder{
		m:         map[trace.TraceID]*stubs{},
		listeners: map[trace.TraceID]int{},
	}

	go func() {
		t := time.NewTimer(60 * time.Second)
		for {
			<-t.C
			tr.gc()
			t.Reset(50 * time.Second)
		}
	}()

	return tr
}

func (r *TraceRecorder) Record(traceID trace.TraceID) func() []tracetest.SpanStub {
	if r.flush != nil {
		r.flush(context.TODO())
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.listeners[traceID]++
	var once sync.Once
	var spans []tracetest.SpanStub
	return func() []tracetest.SpanStub {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()

			if v, ok := r.m[traceID]; ok {
				spans = v.spans
			}
			r.listeners[traceID]--
			if r.listeners[traceID] == 0 {
				delete(r.listeners, traceID)
			}
		})
		return spans
	}
}

func (r *TraceRecorder) gc() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for k, s := range r.m {
		if _, ok := r.listeners[k]; ok {
			continue
		}
		if now.Sub(s.last) > 60*time.Second {
			delete(r.m, k)
		}
	}
}

func (r *TraceRecorder) ExportSpans(ctx context.Context, spans []sdktrace.ReadOnlySpan) error {
	r.mu.Lock()

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
	r.mu.Unlock()

	if r.SpanExporter == nil {
		return nil
	}
	return r.SpanExporter.ExportSpans(ctx, spans)
}

func (r *TraceRecorder) Shutdown(ctx context.Context) error {
	if r.SpanExporter == nil {
		return nil
	}
	return r.SpanExporter.Shutdown(ctx)
}
