package main

import (
	"context"
	"strconv"

	"github.com/containerd/containerd/log"
	"github.com/moby/buildkit/util/tracing/detect"
	"github.com/pkg/errors"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// See https://opentelemetry.io/docs/specs/otel/configuration/sdk-environment-variables/ for details on env vars/values.
const (
	otelSDKDisabledEnv                = "OTEL_SDK_DISABLED"
	otelTracesExporterEnv             = "OTEL_TRACES_EXPORTER"
	otelExporterOTLPEndpointEnv       = "OTEL_EXPORTER_OTLP_ENDPOINT"
	otelExporterOTLPTracesEndpointEnv = "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"
	otelExporterOTLPTracesProtocol    = "OTEL_EXPORTER_OTLP_TRACES_PROTOCOL"
	otelExporterOTLPProtocolEnv       = "OTEL_EXPORTER_OTLP_PROTOCOL"
	otelServiceNameEnv                = "OTEL_SERVICE_NAME"
	otelTracesSamplerEnv              = "OTEL_TRACES_SAMPLER"
	otelTracesSamplerArgEnv           = "OTEL_TRACES_SAMPLER_ARG"
)

var errTracingDisabled = errors.New("tracing disabled")

func getTracerProvider(ctx context.Context, getEnv func(string) string) (*sdktrace.TracerProvider, error) {
	// By default the OTLP libs will connect to localhost if no endpoint is set.
	// This is undesirable for our case since dockerd typically runs as a system
	// service, it shouldn't be connecting to an unprivileged port without
	// explicit configuration.

	if v := getEnv(otelSDKDisabledEnv); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil || b {
			if err != nil {
				err = errors.Wrap(errTracingDisabled, errors.Wrap(err, "failed to parse env").Error())
			} else {
				err = errors.Wrap(errTracingDisabled, "tracing disabled by env")
			}
			return nil, errors.Wrapf(err, "%s=%s", otelSDKDisabledEnv, v)
		}
	}

	// We default to otlp, any other value than empty or "none" is unsupported
	expName := getEnv(otelTracesExporterEnv)
	switch expName {
	case "otlp", "":
	case "none":
		return nil, errors.Wrapf(errTracingDisabled, "trace exports disabled by env %s=%s", otelTracesExporterEnv, expName)
	default:
		return nil, errors.Errorf("unsupported tracing exporter %s in env %s", expName, otelTracesExporterEnv)
	}

	if expName == "" {
		// Only check if the endpoint vars are set if the exporter is not explicitly set
		// This means if the expertor name is set but the endpoints are not set it will use the default from the otel lib.
		if getEnv(otelExporterOTLPEndpointEnv) == "" && getEnv(otelExporterOTLPTracesEndpointEnv) == "" {
			log.G(ctx).Debug("No tracing endpoint configured, skipping")
			return nil, errors.Wrap(errTracingDisabled, "no tracing endpoint configured")
		}
	}

	var (
		exp *otlptrace.Exporter
		err error
	)

	proto := getEnv(otelExporterOTLPTracesProtocol)
	if proto == "" {
		proto = getEnv(otelExporterOTLPProtocolEnv)
	}

	switch proto {
	case "grpc":
		exp, err = otlptracegrpc.New(ctx)
	case "http/protobuf", "":
		exp, err = otlptracehttp.New(ctx)
	default:
		return nil, errors.New("unsupported otlp protocol, only grpc and http/protobuf are supported")
	}

	if err != nil {
		return nil, errors.Wrap(err, "failed to create otlp exporter")
	}

	detect.Register("otlp", func() (sdktrace.SpanExporter, error) {
		return exp, err
	}, 0)

	sp := sdktrace.NewBatchSpanProcessor(exp)

	var sampler sdktrace.Sampler

	samplerValue := getEnv(otelTracesSamplerEnv)
	switch samplerValue {
	case "always_on":
		sampler = sdktrace.AlwaysSample()
	case "always_off":
		sampler = sdktrace.NeverSample()
	case "parentbased_always_on", "":
		sampler = sdktrace.ParentBased(sdktrace.AlwaysSample())
	case "parentbased_always_off":
		sampler = sdktrace.ParentBased(sdktrace.NeverSample())
	case "parentbased_traceidratio":
		defaultRatio := 1.0
		ratio := defaultRatio
		if v := getEnv(otelTracesSamplerArgEnv); v != "" {
			f, err := strconv.ParseFloat(v, 64)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to parse %s=%s", otelTracesSamplerArgEnv, v)
			} else {
				ratio = f
			}
		}

		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))
	default:
		log.G(ctx).WithField("sampler", samplerValue).Warn("Unsupported tracing sampler, using parentbased_always_on")
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(1.0))
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sp), sdktrace.WithSampler(sampler))

	return tp, nil
}
