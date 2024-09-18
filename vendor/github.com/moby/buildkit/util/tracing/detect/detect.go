package detect

import (
	"context"
	"os"
	"sort"
	"strconv"

	"github.com/pkg/errors"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type ExporterDetector interface {
	DetectTraceExporter() (sdktrace.SpanExporter, error)
	DetectMetricExporter() (sdkmetric.Exporter, error)
}

type detector struct {
	f        ExporterDetector
	priority int
}

var detectors map[string]detector

func Register(name string, exp ExporterDetector, priority int) {
	if detectors == nil {
		detectors = map[string]detector{}
	}
	detectors[name] = detector{
		f:        exp,
		priority: priority,
	}
}

type TraceExporterDetector func() (sdktrace.SpanExporter, error)

func (fn TraceExporterDetector) DetectTraceExporter() (sdktrace.SpanExporter, error) {
	return fn()
}

func (fn TraceExporterDetector) DetectMetricExporter() (sdkmetric.Exporter, error) {
	return nil, nil
}

func detectExporter[T any](envVar string, fn func(d ExporterDetector) (T, bool, error)) (exp T, err error) {
	ignoreErrors, _ := strconv.ParseBool("OTEL_IGNORE_ERROR")

	if n := os.Getenv(envVar); n != "" {
		d, ok := detectors[n]
		if !ok {
			if !ignoreErrors {
				err = errors.Errorf("unsupported opentelemetry exporter %v", n)
			}
			return exp, err
		}
		exp, _, err = fn(d.f)
		if err != nil && ignoreErrors {
			err = nil
		}
		return exp, err
	}

	arr := make([]detector, 0, len(detectors))
	for _, d := range detectors {
		arr = append(arr, d)
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].priority < arr[j].priority
	})

	var ok bool
	for _, d := range arr {
		exp, ok, err = fn(d.f)
		if err != nil && !ignoreErrors {
			return exp, err
		}

		if ok {
			break
		}
	}
	return exp, nil
}

func NewSpanExporter(_ context.Context) (sdktrace.SpanExporter, error) {
	return detectExporter("OTEL_TRACES_EXPORTER", func(d ExporterDetector) (sdktrace.SpanExporter, bool, error) {
		exp, err := d.DetectTraceExporter()
		return exp, exp != nil, err
	})
}

func NewMetricExporter(_ context.Context) (sdkmetric.Exporter, error) {
	return detectExporter("OTEL_METRICS_EXPORTER", func(d ExporterDetector) (sdkmetric.Exporter, bool, error) {
		exp, err := d.DetectMetricExporter()
		return exp, exp != nil, err
	})
}

type noneDetector struct{}

func (n noneDetector) DetectTraceExporter() (sdktrace.SpanExporter, error) {
	return noneSpanExporter{}, nil
}

func (n noneDetector) DetectMetricExporter() (sdkmetric.Exporter, error) {
	return noneMetricExporter{}, nil
}

type noneSpanExporter struct{}

func (n noneSpanExporter) ExportSpans(_ context.Context, _ []sdktrace.ReadOnlySpan) error {
	return nil
}

func (n noneSpanExporter) Shutdown(_ context.Context) error {
	return nil
}

func IsNoneSpanExporter(exp sdktrace.SpanExporter) bool {
	_, ok := exp.(noneSpanExporter)
	return ok
}

type noneMetricExporter struct{}

func (n noneMetricExporter) Temporality(kind sdkmetric.InstrumentKind) metricdata.Temporality {
	return sdkmetric.DefaultTemporalitySelector(kind)
}

func (n noneMetricExporter) Aggregation(kind sdkmetric.InstrumentKind) sdkmetric.Aggregation {
	return sdkmetric.DefaultAggregationSelector(kind)
}

func (n noneMetricExporter) Export(_ context.Context, _ *metricdata.ResourceMetrics) error {
	return nil
}

func (n noneMetricExporter) ForceFlush(_ context.Context) error {
	return nil
}

func (n noneMetricExporter) Shutdown(_ context.Context) error {
	return nil
}

func IsNoneMetricExporter(exp sdkmetric.Exporter) bool {
	_, ok := exp.(noneMetricExporter)
	return ok
}

func init() {
	// Register a none detector. This will never be chosen if there's another suitable
	// exporter that can be detected, but exists to allow telemetry to be explicitly
	// disabled.
	Register("none", noneDetector{}, 1000)
}
