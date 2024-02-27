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
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type ExporterDetector func() (sdktrace.SpanExporter, sdkmetric.Exporter, error)

type detector struct {
	f        ExporterDetector
	priority int
}

var ServiceName string
var Recorder *TraceRecorder

var detectors map[string]detector
var once sync.Once
var tp trace.TracerProvider
var mp metric.MeterProvider
var exporter struct {
	SpanExporter   sdktrace.SpanExporter
	MetricExporter sdkmetric.Exporter
}
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

func detectExporter() (texp sdktrace.SpanExporter, mexp sdkmetric.Exporter, err error) {
	if n := os.Getenv("OTEL_TRACES_EXPORTER"); n != "" {
		d, ok := detectors[n]
		if !ok {
			if n == "none" {
				return nil, nil, nil
			}
			return nil, nil, errors.Errorf("unsupported opentelemetry tracer %v", n)
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
		t, m, err := d.f()
		if err != nil {
			return nil, nil, err
		}
		if texp == nil {
			texp = t
		}
		if mexp == nil {
			mexp = m
		}

		// Found a candidate for both exporters so just return now.
		if texp != nil && mexp != nil {
			return texp, mexp, nil
		}
	}
	return texp, mexp, nil
}

func getExporters() (sdktrace.SpanExporter, sdkmetric.Exporter, error) {
	texp, mexp, err := detectExporter()
	if err != nil {
		return nil, nil, err
	}

	if Recorder != nil {
		Recorder.SpanExporter = texp
		texp = Recorder
	}

	return texp, mexp, nil
}

func detect() error {
	tp = noop.NewTracerProvider()
	mp = sdkmetric.NewMeterProvider()

	texp, mexp, err := getExporters()
	if err != nil || (texp == nil && mexp == nil) {
		return err
	}

	res := Resource()

	// enable log with traceID when valid exporter
	if texp != nil {
		bklog.EnableLogWithTraceID(true)

		sp := sdktrace.NewBatchSpanProcessor(texp)

		if Recorder != nil {
			Recorder.flush = sp.ForceFlush
		}

		sdktp := sdktrace.NewTracerProvider(
			sdktrace.WithSpanProcessor(sp),
			sdktrace.WithResource(res),
		)
		closers = append(closers, sdktp.Shutdown)

		exporter.SpanExporter = texp
		tp = sdktp
	}

	var readers []sdkmetric.Reader
	if mexp != nil {
		// Create a new periodic reader using any configured metric exporter.
		readers = append(readers, sdkmetric.NewPeriodicReader(mexp))
	}

	if r, err := prometheus.New(); err != nil {
		// Log the error but do not fail if we could not configure the prometheus metrics.
		bklog.G(context.Background()).
			WithError(err).
			Error("failed prometheus metrics configuration")
	} else {
		// Register the prometheus reader if there was no error.
		readers = append(readers, r)
	}

	if len(readers) > 0 {
		opts := make([]sdkmetric.Option, 0, len(readers)+1)
		opts = append(opts, sdkmetric.WithResource(res))
		for _, r := range readers {
			opts = append(opts, sdkmetric.WithReader(r))
		}
		sdkmp := sdkmetric.NewMeterProvider(opts...)
		closers = append(closers, sdkmp.Shutdown)

		exporter.MetricExporter = mexp
		mp = sdkmp
	}
	return nil
}

func TracerProvider() (trace.TracerProvider, error) {
	if err := detectOnce(); err != nil {
		return nil, err
	}
	return tp, nil
}

func MeterProvider() (metric.MeterProvider, error) {
	if err := detectOnce(); err != nil {
		return nil, err
	}
	return mp, nil
}

func detectOnce() error {
	once.Do(func() {
		if err1 := detect(); err1 != nil {
			b, _ := strconv.ParseBool(os.Getenv("OTEL_IGNORE_ERROR"))
			if !b {
				err = err1
			}
		}
	})
	return err
}

func Exporter() (sdktrace.SpanExporter, sdkmetric.Exporter, error) {
	_, err := TracerProvider()
	if err != nil {
		return nil, nil, err
	}
	return exporter.SpanExporter, exporter.MetricExporter, nil
}

func Shutdown(ctx context.Context) error {
	for _, c := range closers {
		if err := c(ctx); err != nil {
			return err
		}
	}
	return nil
}

var (
	detectedResource     *resource.Resource
	detectedResourceOnce sync.Once
)

func Resource() *resource.Resource {
	detectedResourceOnce.Do(func() {
		res, err := resource.New(context.Background(),
			resource.WithDetectors(serviceNameDetector{}),
			resource.WithFromEnv(),
			resource.WithTelemetrySDK(),
		)
		if err != nil {
			otel.Handle(err)
		}
		detectedResource = res
	})
	return detectedResource
}

// OverrideResource overrides the resource returned from Resource.
//
// This must be invoked before Resource is called otherwise it is a no-op.
func OverrideResource(res *resource.Resource) {
	detectedResourceOnce.Do(func() {
		detectedResource = res
	})
}

type serviceNameDetector struct{}

func (serviceNameDetector) Detect(ctx context.Context) (*resource.Resource, error) {
	return resource.StringDetector(
		semconv.SchemaURL,
		semconv.ServiceNameKey,
		func() (string, error) {
			if ServiceName != "" {
				return ServiceName, nil
			}
			return filepath.Base(os.Args[0]), nil
		},
	).Detect(ctx)
}
