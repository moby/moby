package testutil // import "github.com/docker/docker/testutil"

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/containerd/log"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"gotest.tools/v3/icmd"
)

// DevZero acts like /dev/zero but in an OS-independent fashion.
var DevZero io.Reader = devZero{}

type devZero struct{}

func (d devZero) Read(p []byte) (n int, err error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

var tracingOnce sync.Once

// configureTracing sets up an OTLP tracing exporter for use in tests.
func ConfigureTracing() func(context.Context) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		// No OTLP endpoint configured, so don't bother setting up tracing.
		// Since we are not using a batch exporter we don't want tracing to block up tests.
		return func(context.Context) {}
	}
	var tp *trace.TracerProvider
	tracingOnce.Do(func() {
		ctx := context.Background()
		exp := otlptracehttp.NewUnstarted()
		sp := trace.NewBatchSpanProcessor(exp)
		props := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
		otel.SetTextMapPropagator(props)

		tp = trace.NewTracerProvider(
			trace.WithSpanProcessor(sp),
			trace.WithSampler(trace.AlwaysSample()),
			trace.WithResource(resource.NewSchemaless(
				attribute.KeyValue{Key: semconv.ServiceNameKey, Value: attribute.StringValue("integration-test-client")},
			)),
		)
		otel.SetTracerProvider(tp)

		if err := exp.Start(ctx); err != nil {
			log.G(ctx).WithError(err).Warn("Failed to start tracing exporter")
		}
	})

	// if ConfigureTracing was called multiple times we'd have a nil `tp` here
	// Get the already configured tracer provider
	if tp == nil {
		tp = otel.GetTracerProvider().(*trace.TracerProvider)
	}
	return func(ctx context.Context) {
		if err := tp.Shutdown(ctx); err != nil {
			log.G(ctx).WithError(err).Warn("Failed to shutdown tracer")
		}
	}
}

// TestingT is an interface wrapper around *testing.T and *testing.B.
type TestingT interface {
	Name() string
	Cleanup(func())
	Log(...any)
	Failed() bool
}

// StartSpan starts a span for the given test.
func StartSpan(ctx context.Context, t TestingT) context.Context {
	ConfigureTracing()
	ctx, span := otel.Tracer("").Start(ctx, t.Name())
	t.Cleanup(func() {
		if t.Failed() {
			span.SetStatus(codes.Error, "test failed")
		}
		span.End()
	})
	return ctx
}

func RunCommand(ctx context.Context, cmd string, args ...string) *icmd.Result {
	_, span := otel.Tracer("").Start(ctx, "RunCommand "+cmd+" "+strings.Join(args, " "))
	res := icmd.RunCommand(cmd, args...)
	if res.Error != nil {
		span.SetStatus(codes.Error, res.Error.Error())
	}
	span.SetAttributes(attribute.String("cmd", cmd), attribute.String("args", strings.Join(args, " ")))
	span.SetAttributes(attribute.Int("exit", res.ExitCode))
	span.SetAttributes(attribute.String("stdout", res.Stdout()), attribute.String("stderr", res.Stderr()))
	span.End()
	return res
}

type testContextStore struct {
	mu  sync.Mutex
	idx map[TestingT]context.Context
}

var testContexts = &testContextStore{idx: make(map[TestingT]context.Context)}

func (s *testContextStore) Get(t TestingT) context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx, ok := s.idx[t]
	if ok {
		return ctx
	}
	ctx = context.Background()
	s.idx[t] = ctx
	return ctx
}

func (s *testContextStore) Set(ctx context.Context, t TestingT) {
	s.mu.Lock()
	if _, ok := s.idx[t]; ok {
		panic("test context already set")
	}
	s.idx[t] = ctx
	s.mu.Unlock()
}

func (s *testContextStore) Delete(t *testing.T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.idx, t)
}

func GetContext(t TestingT) context.Context {
	return testContexts.Get(t)
}

func SetContext(t TestingT, ctx context.Context) {
	testContexts.Set(ctx, t)
}

func CleanupContext(t *testing.T) {
	testContexts.Delete(t)
}
