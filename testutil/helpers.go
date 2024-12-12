// FIXME(thaJeztah): remove once we are a module; the go:build directive prevents go from downgrading language version to go1.16:
//go:build go1.22

package testutil // import "github.com/docker/docker/testutil"

import (
	"context"
	"io"
	"os"
	"reflect"
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
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace/noop"
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

// ConfigureTracing sets up an OTLP tracing exporter for use in tests.
func ConfigureTracing() func(context.Context) {
	if os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") == "" {
		// No OTLP endpoint configured, so don't bother setting up tracing.
		// Since we are not using a batch exporter we don't want tracing to block up tests.
		otel.SetTracerProvider(noop.NewTracerProvider())
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
			trace.WithResource(resource.NewSchemaless(semconv.ServiceName("integration-test-client"))),
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

// CheckNotParallel checks if t.Parallel() was not called on the current test.
// There's no public method to check this, so we use reflection to check the
// internal field set by t.Parallel()
// https://github.com/golang/go/blob/8e658eee9c7a67a8a79a8308695920ac9917566c/src/testing/testing.go#L1449
//
// Since this is not a public API, it might change at any time.
func CheckNotParallel(t testing.TB) {
	t.Helper()
	field := reflect.ValueOf(t).Elem().FieldByName("isParallel")
	if field.IsValid() {
		if field.Bool() {
			t.Fatal("t.Parallel() was called before")
		}
	} else {
		t.Logf("FIXME: CheckParallel could not determine if test %s is parallel - did the t.Parallel() implementation change?", t.Name())
	}
}
