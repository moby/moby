package detect

import (
	"context"
	"os"

	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func init() {
	Register("otlp", otlpExporter, 10)
}

func otlpExporter() (sdktrace.SpanExporter, error) {
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
