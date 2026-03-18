package detect

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var otlpExporter = otlpExporterDetector{}

func init() {
	Register("otlp", otlpExporter, 10)
}

type otlpExporterDetector struct{}

func (otlpExporterDetector) DetectTraceExporter() (sdktrace.SpanExporter, error) {
	set := os.Getenv("OTEL_TRACES_EXPORTER") == "otlp" || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT") != ""
	if !set {
		return nil, nil
	}

	proto := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if proto == "" {
		proto = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}
	if proto == "" {
		proto = "grpc"
	}

	var c otlptrace.Client

	switch proto {
	case "grpc":
		c = otlptracegrpc.NewClient()
	case "http/protobuf":
		c = otlptracehttp.NewClient()
	// case "http/json": // unsupported by library
	default:
		return nil, errors.Errorf("unsupported otlp protocol %v", proto)
	}

	return otlptrace.New(context.Background(), c)
}

func (otlpExporterDetector) DetectMetricExporter() (sdkmetric.Exporter, error) {
	set := os.Getenv("OTEL_METRICS_EXPORTER") == "otlp" || os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "" || os.Getenv("OTEL_EXPORTER_OTLP_METRICS_ENDPOINT") != ""
	if !set {
		return nil, nil
	}

	proto := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")
	if proto == "" {
		proto = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}
	if proto == "" {
		proto = "grpc"
	}

	switch proto {
	case "grpc":
		return otlpmetricgrpc.New(context.Background(),
			otlpmetricgrpc.WithTemporalitySelector(deltaTemporality),
		)
	case "http/protobuf":
		return otlpmetrichttp.New(context.Background(),
			otlpmetrichttp.WithTemporalitySelector(deltaTemporality),
		)
	// case "http/json": // unsupported by library
	default:
		return nil, errors.Errorf("unsupported otlp protocol %v", proto)
	}
}

func deltaTemporality(_ sdkmetric.InstrumentKind) metricdata.Temporality {
	return metricdata.DeltaTemporality
}
