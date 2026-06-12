// SPDX-FileCopyrightText: Copyright 2015-2025 go-swagger maintainers
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"
)

const (
	instrumentationVersion = "1.0.0"
	tracerName             = "go-openapi"
)

// WithOpenTelemetry adds opentelemetry support to the provided runtime.
// A new client span is created for each request.
// The provided opts are applied to each spans - for example to add global tags.
//
// The returned transport satisfies [runtime.ContextualTransport]: callers
// should prefer [openTelemetryTransport.SubmitContext] over the
// legacy [runtime.ClientOperation.Context] field. Setting that
// field is still honored on the [openTelemetryTransport.Submit]
// compatibility path.
func (r *Runtime) WithOpenTelemetry(opts ...OpenTelemetryOpt) runtime.ContextualTransport {
	return newOpenTelemetryTransport(r, r.Host, opts)
}

// WithOpenTracing adds opentracing support to the provided runtime.
// A new client span is created for each request.
// If the context of the client operation does not contain an active span, no span is created.
// The provided opts are applied to each spans - for example to add global tags.
//
// Deprecated: use [WithOpenTelemetry] instead, as opentracing is now archived and superseded by opentelemetry.
//
// # Deprecation notice
//
// The [Runtime.WithOpenTracing] method has been deprecated in favor of [Runtime.WithOpenTelemetry].
//
// The method is still around so programs calling it will still build. However, it will return
// an opentelemetry transport.
//
// If you have a strict requirement on using opentracing, you may still do so by importing
// module [github.com/go-openapi/runtime/client-[middleware]/opentracing] and using
// [github.com/go-openapi/runtime/client-[middleware]/opentracing.WithOpenTracing] with your
// usual opentracing options and opentracing-enabled transport.
//
// Passed options are ignored unless they are of type [OpenTelemetryOpt].
func (r *Runtime) WithOpenTracing(opts ...any) runtime.ContextualTransport {
	otelOpts := make([]OpenTelemetryOpt, 0, len(opts))
	for _, o := range opts {
		otelOpt, ok := o.(OpenTelemetryOpt)
		if !ok {
			continue
		}
		otelOpts = append(otelOpts, otelOpt)
	}

	return r.WithOpenTelemetry(otelOpts...)
}

type config struct {
	Tracer            trace.Tracer
	Propagator        propagation.TextMapPropagator
	SpanStartOptions  []trace.SpanStartOption
	SpanNameFormatter func(*runtime.ClientOperation) string
	TracerProvider    trace.TracerProvider
}

type OpenTelemetryOpt interface {
	apply(*config)
}

type optionFunc func(*config)

func (o optionFunc) apply(c *config) {
	o(c)
}

// WithTracerProvider specifies a tracer provider to use for creating a tracer.
// If none is specified, the global provider is used.
func WithTracerProvider(provider trace.TracerProvider) OpenTelemetryOpt {
	return optionFunc(func(c *config) {
		if provider != nil {
			c.TracerProvider = provider
		}
	})
}

// WithPropagators configures specific propagators. If this
// option isn't specified, then the global TextMapPropagator is used.
func WithPropagators(ps propagation.TextMapPropagator) OpenTelemetryOpt {
	return optionFunc(func(c *config) {
		if ps != nil {
			c.Propagator = ps
		}
	})
}

// WithSpanOptions configures an additional set of
// trace.SpanOptions, which are applied to each new span.
func WithSpanOptions(opts ...trace.SpanStartOption) OpenTelemetryOpt {
	return optionFunc(func(c *config) {
		c.SpanStartOptions = append(c.SpanStartOptions, opts...)
	})
}

// WithSpanNameFormatter takes a function that will be called on every
// request and the returned string will become the Span Name.
func WithSpanNameFormatter(f func(op *runtime.ClientOperation) string) OpenTelemetryOpt {
	return optionFunc(func(c *config) {
		c.SpanNameFormatter = f
	})
}

func defaultTransportFormatter(op *runtime.ClientOperation) string {
	if op.ID != "" {
		return op.ID
	}

	return fmt.Sprintf("%s_%s", strings.ToLower(op.Method), op.PathPattern)
}

type openTelemetryTransport struct {
	transport runtime.ClientTransport
	host      string
	tracer    trace.Tracer
	config    *config
}

func newOpenTelemetryTransport(transport runtime.ClientTransport, host string, opts []OpenTelemetryOpt) *openTelemetryTransport {
	tr := &openTelemetryTransport{
		transport: transport,
		host:      host,
	}

	const baseOptions = 4
	defaultOpts := make([]OpenTelemetryOpt, 0, len(opts)+baseOptions)
	defaultOpts = append(defaultOpts,
		WithSpanOptions(trace.WithSpanKind(trace.SpanKindClient)),
		WithSpanNameFormatter(defaultTransportFormatter),
		WithPropagators(otel.GetTextMapPropagator()),
		WithTracerProvider(otel.GetTracerProvider()),
	)

	c := newConfig(append(defaultOpts, opts...)...)
	tr.config = c

	return tr
}

// Submit implements [runtime.ClientTransport]. It honors the legacy
// [runtime.ClientOperation.Context] field for backward compatibility
// — that field is being phased out; new code should call
// [openTelemetryTransport.SubmitContext] directly with an explicit
// context.
func (t *openTelemetryTransport) Submit(op *runtime.ClientOperation) (any, error) {
	ctx := op.Context
	if ctx == nil {
		ctx = context.Background()
	}
	return t.SubmitContext(ctx, op)
}

// SubmitContext submits an operation with an explicit context that
// drives both the tracing span and (when supported) the wrapped
// transport's SubmitContext call. The legacy
// [runtime.ClientOperation.Context] field is not consulted.
//
// When the wrapped transport implements [runtime.ContextualTransport], ctx is
// forwarded directly via its SubmitContext. Otherwise, the legacy
// Submit path is used: ctx is stamped onto op.Context for the
// duration of that call and restored afterwards, so the wrapped
// transport still receives a usable context. The legacy fallback
// disappears once SubmitContext is universal (v2).
func (t *openTelemetryTransport) SubmitContext(ctx context.Context, op *runtime.ClientOperation) (any, error) {
	params := op.Params
	reader := op.Reader

	var span trace.Span
	defer func() {
		if span != nil {
			span.End()
		}
	}()

	op.Params = runtime.ClientRequestWriterFunc(func(req runtime.ClientRequest, reg strfmt.Registry) error {
		span = t.newOpenTelemetrySpan(ctx, op, req.GetHeaderParams())
		return params.WriteToRequest(req, reg)
	})

	op.Reader = runtime.ClientResponseReaderFunc(func(response runtime.ClientResponse, consumer runtime.Consumer) (any, error) {
		if span != nil {
			statusCode := response.Code()
			// NOTE: this is replaced by semconv.HTTPResponseStatusCode in semconv v1.21
			span.SetAttributes(semconv.HTTPResponseStatusCode(statusCode))
			// NOTE: the conversion from HTTP status code to trace code is no longer available with
			// semconv v1.21
			const minHTTPStatusIsError = 400
			if statusCode >= minHTTPStatusIsError {
				span.SetStatus(codes.Error, http.StatusText(statusCode))
			}
		}

		return reader.ReadResponse(response, consumer)
	})

	submit, err := t.submitWrapped(ctx, op)
	if err != nil && span != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}

	return submit, err
}

//nolint:contextcheck // ctx is forwarded verbatim; the legacy Submit branch only stamps it onto op.Context for the wrapped transport.
func (t *openTelemetryTransport) submitWrapped(ctx context.Context, op *runtime.ClientOperation) (any, error) {
	if sc, ok := t.transport.(runtime.ContextualTransport); ok {
		return sc.SubmitContext(ctx, op)
	}
	prev := op.Context
	op.Context = ctx
	defer func() { op.Context = prev }()
	return t.transport.Submit(op)
}

func (t *openTelemetryTransport) newOpenTelemetrySpan(ctx context.Context, op *runtime.ClientOperation, header http.Header) trace.Span {
	tracer := t.tracer
	if tracer == nil {
		if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
			tracer = newTracer(span.TracerProvider())
		} else {
			tracer = newTracer(otel.GetTracerProvider())
		}
	}

	ctx, span := tracer.Start(ctx, t.config.SpanNameFormatter(op), t.config.SpanStartOptions...)

	var scheme string
	if len(op.Schemes) > 0 {
		scheme = op.Schemes[0]
	}

	span.SetAttributes(
		attribute.String("net.peer.name", t.host),
		attribute.String(string(semconv.HTTPRouteKey), op.PathPattern),
		attribute.String(string(semconv.HTTPRequestMethodKey), op.Method),
		attribute.String("span.kind", trace.SpanKindClient.String()),
		attribute.String("http.scheme", scheme),
	)

	carrier := propagation.HeaderCarrier(header)
	t.config.Propagator.Inject(ctx, carrier)

	return span
}

func newTracer(tp trace.TracerProvider) trace.Tracer {
	return tp.Tracer(tracerName, trace.WithInstrumentationVersion(version()))
}

func newConfig(opts ...OpenTelemetryOpt) *config {
	c := &config{
		Propagator: otel.GetTextMapPropagator(),
	}

	for _, opt := range opts {
		opt.apply(c)
	}

	// Tracer is only initialized if manually specified. Otherwise, can be passed with the tracing context.
	if c.TracerProvider != nil {
		c.Tracer = newTracer(c.TracerProvider)
	}

	return c
}

// Version is the current release version of the go-runtime instrumentation.
func version() string {
	return instrumentationVersion
}
