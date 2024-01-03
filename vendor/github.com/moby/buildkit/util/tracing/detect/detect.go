package detect

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"

	"github.com/moby/buildkit/util/bklog"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
)

type ExporterDetector func() (sdktrace.SpanExporter, error)

type detector struct {
	f        ExporterDetector
	priority int
}

var ServiceName string
var Recorder *TraceRecorder

var Resource *resource.Resource

var detectors map[string]detector
var once sync.Once
var tp trace.TracerProvider
var exporter sdktrace.SpanExporter
var closers []func(context.Context) error
var err error

func Register(name string, exp ExporterDetector, priority int) {
	if detectors == nil {
		detectors = map[string]detector{}
	}
	detectors[name] = detector{
		f:        exp,
		priority: priority,
	}
}

func detectExporter() (sdktrace.SpanExporter, error) {
	if n := os.Getenv("OTEL_TRACES_EXPORTER"); n != "" {
		d, ok := detectors[n]
		if !ok {
			if n == "none" {
				return nil, nil
			}
			return nil, errors.Errorf("unsupported opentelemetry tracer %v", n)
		}
		return d.f()
	}
	arr := make([]detector, 0, len(detectors))
	for _, d := range detectors {
		arr = append(arr, d)
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].priority < arr[j].priority
	})
	for _, d := range arr {
		exp, err := d.f()
		if err != nil {
			return nil, err
		}
		if exp != nil {
			return exp, nil
		}
	}
	return nil, nil
}

func getExporter() (sdktrace.SpanExporter, error) {
	exp, err := detectExporter()
	if err != nil {
		return nil, err
	}

	if Recorder != nil {
		Recorder.SpanExporter = exp
		exp = Recorder
	}
	return exp, nil
}

func detect() error {
	tp = trace.NewNoopTracerProvider()

	exp, err := getExporter()
	if err != nil || exp == nil {
		return err
	}

	// enable log with traceID when valid exporter
	bklog.EnableLogWithTraceID(true)

	if Resource == nil {
		res, err := resource.Detect(context.Background(), serviceNameDetector{})
		if err != nil {
			return err
		}
		res, err = resource.Merge(resource.Default(), res)
		if err != nil {
			return err
		}
		Resource = res
	}

	sp := sdktrace.NewBatchSpanProcessor(exp)

	if Recorder != nil {
		Recorder.flush = sp.ForceFlush
	}

	sdktp := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(sp),
		sdktrace.WithResource(Resource),
	)
	closers = append(closers, sdktp.Shutdown)

	exporter = exp
	tp = sdktp
	return nil
}

func TracerProvider() (trace.TracerProvider, error) {
	once.Do(func() {
		if err1 := detect(); err1 != nil {
			err = err1
		}
	})
	b, _ := strconv.ParseBool(os.Getenv("OTEL_IGNORE_ERROR"))
	if err != nil && !b {
		return nil, err
	}
	return tp, nil
}

func Exporter() (sdktrace.SpanExporter, error) {
	_, err := TracerProvider()
	if err != nil {
		return nil, err
	}
	return exporter, nil
}

func Shutdown(ctx context.Context) error {
	for _, c := range closers {
		if err := c(ctx); err != nil {
			return err
		}
	}
	return nil
}

type serviceNameDetector struct{}

func (serviceNameDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.StringDetector(
		semconv.SchemaURL,
		semconv.ServiceNameKey,
		func() (string, error) {
			if n := os.Getenv("OTEL_SERVICE_NAME"); n != "" {
				return n, nil
			}
			if ServiceName != "" {
				return ServiceName, nil
			}
			return filepath.Base(os.Args[0]), nil
		},
	).Detect(ctx)
}
